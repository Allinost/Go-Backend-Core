// Command v3ledger 解析 V3 项目台账 Markdown 并幂等导入 PM 表。
//
//	go run ./cmd/v3ledger -file docs/projects/go-backend-core/v3/README.md
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Allinost/go-backend-core/internal/config"
	"github.com/Allinost/go-backend-core/internal/database/postgres"
	dm "github.com/Allinost/go-backend-core/zzz/developmentManager"
)

type ledgerEntry struct {
	Code       string
	Title      string
	Type       string
	Priority   string
	Estimate   float64
	Deps       []string
	Status     string
	Acceptance string
	OrderNo    int
}

var (
	depCodeRe  = regexp.MustCompile(`(?:V3-)?P\d+-\d+`)
	depRangeRe = regexp.MustCompile(`(?:V3-)?P(\d+)-(\d+)\.\.(\d+)`)
)

func main() {
	file := flag.String("file", "docs/projects/go-backend-core/v3/README.md", "台账 Markdown 文件")
	projectKey := flag.String("project", "go-backend-core", "项目唯一键")
	milestoneVersion := flag.String("milestone", "V3.0", "里程碑版本")
	dryRun := flag.Bool("dry-run", false, "只解析并打印统计，不写库")
	flag.Parse()

	entries, err := parseLedger(*file)
	if err != nil {
		fail("解析台账失败: " + err.Error())
	}
	if len(entries) == 0 {
		fail("未解析到 V3 任务")
	}
	fmt.Printf("解析任务：%d 个\n", len(entries))
	if *dryRun {
		return
	}

	cfg, err := config.Load("config.yaml")
	if err != nil {
		fail("加载配置失败: " + err.Error())
	}
	pool, err := postgres.NewPool(cfg.Database.Postgres["main"], false)
	if err != nil {
		fail("连接 PostgreSQL 失败: " + err.Error())
	}
	defer pool.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	repo := dm.NewRepository(pool.SQLDB)
	project, err := repo.GetProjectByKey(ctx, *projectKey)
	if err != nil {
		project, err = repo.CreateProject(ctx, &dm.Project{
			ProjectKey: *projectKey, Name: "Go-Backend-Core", Type: dm.ProjectTypeBase,
			DefaultBranch: "main", CreatedBy: "v3ledger", UpdatedBy: "v3ledger",
		})
		if err != nil {
			fail("创建项目失败: " + err.Error())
		}
		fmt.Printf("创建项目：%s (id=%d)\n", project.ProjectKey, project.ID)
	}

	pm := dm.NewPMService(dm.NewPMRepository(pool.SQLDB))
	milestone, err := ensureMilestone(ctx, pm, project.ID, *milestoneVersion)
	if err != nil {
		fail("准备里程碑失败: " + err.Error())
	}

	items := make([]dm.ImportTaskItem, 0, len(entries))
	for _, e := range entries {
		title := truncateRunes(e.Title, 255)
		estimate := e.Estimate
		order := e.OrderNo
		items = append(items, dm.ImportTaskItem{
			MilestoneID: &milestone.ID, Code: e.Code, Title: title, Description: e.Acceptance,
			Type: e.Type, Status: e.Status, Priority: e.Priority,
			EstimateHours: &estimate, OrderNo: &order,
		})
	}
	resp, err := pm.ImportTasks(ctx, project.ID, &dm.ImportTasksReq{Tasks: items}, "v3ledger")
	if err != nil {
		fail("导入任务失败: " + err.Error())
	}
	fmt.Printf("任务导入：created=%d updated=%d skipped=%d\n", resp.Created, resp.Updated, resp.Skipped)

	tasks, _, _, err := pm.ListTasks(ctx, project.ID, &dm.ListTasksReq{Page: 1, PageSize: 100, MilestoneID: &milestone.ID})
	if err != nil {
		fail("读取任务失败: " + err.Error())
	}
	byCode := make(map[string]int64, len(tasks))
	for _, t := range tasks {
		byCode[t.Code] = t.ID
	}

	added, skipped := 0, 0
	for _, e := range entries {
		taskID, ok := byCode[e.Code]
		if !ok {
			continue
		}
		for _, depCode := range e.Deps {
			if depCode == e.Code {
				skipped++
				continue
			}
			depID, ok := byCode[depCode]
			if !ok {
				fmt.Printf("跳过依赖：%s -> %s（任务不存在）\n", e.Code, depCode)
				skipped++
				continue
			}
			_, err := pm.AddDependency(ctx, taskID, &dm.AddTaskDependencyReq{DependsOnTaskID: depID, DepType: "FS"}, "v3ledger")
			if err != nil {
				var dmErr *dm.DevelopmentManagerError
				if errors.As(err, &dmErr) && (dmErr.Code == dm.ErrTaskDependencyExists || dmErr.Code == dm.ErrTaskDependencyCycle) {
					skipped++
					continue
				}
				fmt.Printf("依赖失败：%s -> %s: %v\n", e.Code, depCode, err)
				skipped++
				continue
			}
			added++
		}
	}
	fmt.Printf("依赖导入：added=%d skipped=%d\n", added, skipped)
	fmt.Println("V3 台账导入完成")
}

func ensureMilestone(ctx context.Context, pm *dm.PMService, projectID int64, version string) (*dm.Milestone, error) {
	items, _, _, err := pm.ListMilestones(ctx, projectID, &dm.ListMilestonesReq{Page: 1, PageSize: 100})
	if err != nil {
		return nil, err
	}
	for i := range items {
		if items[i].Version == version {
			return &items[i], nil
		}
	}
	return pm.CreateMilestone(ctx, projectID, &dm.CreateMilestoneReq{
		Version: version, Name: version + " 核心上线", Status: dm.MilestoneStatusActive, TargetDate: "2026-12-15",
	}, "v3ledger")
}

func parseLedger(path string) ([]ledgerEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var entries []ledgerEntry
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "| V3-") {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) < 9 {
			continue
		}
		code := strings.TrimSpace(parts[1])
		if !strings.HasPrefix(code, "V3-") {
			continue
		}
		entry := ledgerEntry{
			Code:       code,
			Title:      cleanMarkdown(parts[2]),
			Type:       strings.TrimSpace(parts[3]),
			Priority:   strings.TrimSpace(parts[4]),
			Status:     mapStatus(strings.TrimSpace(parts[7])),
			Acceptance: cleanMarkdown(parts[8]),
			OrderNo:    len(entries) + 1,
		}
		entry.Estimate, _ = strconv.ParseFloat(strings.TrimSpace(parts[5]), 64)
		entry.Deps = parseDeps(strings.TrimSpace(parts[6]))
		if entry.Type == "" {
			entry.Type = dm.TaskTypeFeature
		}
		if !dm.ValidTaskTypes[entry.Type] {
			entry.Type = dm.TaskTypeFeature
		}
		if !dm.ValidRequirementPriorities[entry.Priority] {
			entry.Priority = dm.PriorityP2
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func parseDeps(raw string) []string {
	raw = strings.ReplaceAll(raw, "，", ",")
	raw = strings.ReplaceAll(raw, "、", ",")
	seen := make(map[string]bool)
	var out []string
	add := func(code string) {
		code = strings.TrimSpace(code)
		if code == "" || seen[code] {
			return
		}
		seen[code] = true
		out = append(out, code)
	}
	for _, m := range depRangeRe.FindAllStringSubmatch(raw, -1) {
		prefix := "V3-"
		if strings.HasPrefix(m[0], "V3-") {
			prefix = "V3-"
		}
		start, _ := strconv.Atoi(m[2])
		end, _ := strconv.Atoi(m[3])
		for n := start; n <= end; n++ {
			add(fmt.Sprintf("%sP%s-%02d", prefix, m[1], n))
		}
	}
	raw = depRangeRe.ReplaceAllString(raw, " ")
	for _, code := range depCodeRe.FindAllString(raw, -1) {
		if !strings.HasPrefix(code, "V3-") {
			code = "V3-" + code
		}
		add(code)
	}
	return out
}

func mapStatus(raw string) string {
	s := strings.ToLower(raw)
	switch {
	case strings.Contains(s, "done"):
		return dm.TaskStatusDone
	case strings.Contains(s, "doing"):
		return dm.TaskStatusDoing
	case strings.Contains(s, "blocked"):
		return dm.TaskStatusBlocked
	default:
		return dm.TaskStatusTodo
	}
}

func cleanMarkdown(s string) string {
	s = strings.ReplaceAll(s, "**", "")
	s = strings.ReplaceAll(s, "`", "")
	s = strings.TrimSpace(s)
	return s
}

func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}

func fail(msg string) {
	fmt.Fprintln(os.Stderr, "v3ledger: "+msg)
	os.Exit(1)
}
