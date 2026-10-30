package developmentmanager

import (
	"context"
	"errors"
	"strings"
	"time"
)

// isDMCode 判断错误是否为指定模块错误码
func isDMCode(err error, code int) bool {
	var de *DevelopmentManagerError
	return errors.As(err, &de) && de.Code == code
}

// requireProject 校验项目存在且未删除
func (s *Service) requireProject(ctx context.Context, projectID int64) (*Project, error) {
	p, err := s.repo.GetProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if p.Deleted {
		return nil, NewDevelopmentManagerError(ErrProjectNotFound)
	}
	return p, nil
}

// requireContainer 校验容器存在且未删除
func (s *Service) requireContainer(ctx context.Context, containerID int64) (*Container, error) {
	c, err := s.repo.GetContainer(ctx, containerID)
	if err != nil {
		return nil, err
	}
	if c.Deleted {
		return nil, NewDevelopmentManagerError(ErrContainerNotFound)
	}
	return c, nil
}

// requireProjectContainer 校验容器存在、未删除且属于指定项目
func (s *Service) requireProjectContainer(ctx context.Context, projectID, containerID int64) (*Container, error) {
	c, err := s.requireContainer(ctx, containerID)
	if err != nil {
		return nil, err
	}
	if c.ProjectID != projectID {
		return nil, NewDevelopmentManagerError(ErrCrossProjectRef)
	}
	return c, nil
}

// ---------- 容器 ----------

// ListContainers 查询项目容器列表
func (s *Service) ListContainers(ctx context.Context, projectID int64, includeDeleted bool) ([]Container, error) {
	if _, err := s.requireProject(ctx, projectID); err != nil {
		return nil, err
	}
	return s.repo.ListContainers(ctx, projectID, includeDeleted)
}

// GetContainer 查询容器详情
func (s *Service) GetContainer(ctx context.Context, id int64) (*Container, error) {
	return s.repo.GetContainer(ctx, id)
}

// validateContainerReq 校验容器请求并归一化
func validateContainerReq(req *CreateContainerReq) (string, error) {
	name := strings.TrimSpace(req.ContainerName)
	if l := len([]rune(name)); l < 1 || l > 128 {
		return "", NewDevelopmentManagerError(ErrInvalidContainerName)
	}
	if req.Replicas < 0 {
		return "", NewDevelopmentManagerError(ErrInvalidReplicas)
	}
	return name, nil
}

// CreateContainer 新建容器
func (s *Service) CreateContainer(ctx context.Context, projectID int64, req *CreateContainerReq, actor string) (*Container, error) {
	if _, err := s.requireProject(ctx, projectID); err != nil {
		return nil, err
	}
	name, err := validateContainerReq(req)
	if err != nil {
		return nil, err
	}
	if _, err := s.repo.GetContainerByName(ctx, projectID, name); err == nil {
		return nil, NewDevelopmentManagerError(ErrDuplicateContainerName)
	} else if !isDMCode(err, ErrContainerNotFound) {
		return nil, err
	}
	replicas := req.Replicas
	if replicas == 0 {
		replicas = 1
	}
	return s.repo.CreateContainer(ctx, &Container{
		ProjectID:     projectID,
		ContainerName: name,
		Image:         strings.TrimSpace(req.Image),
		ComposeRef:    strings.TrimSpace(req.ComposeRef),
		Replicas:      replicas,
		NetworkMode:   strings.TrimSpace(req.NetworkMode),
		EnvNotes:      req.EnvNotes,
		StatusNote:    strings.TrimSpace(req.StatusNote),
		CreatedBy:     actor,
		UpdatedBy:     actor,
	})
}

// UpdateContainer 全量更新容器
func (s *Service) UpdateContainer(ctx context.Context, id int64, req *UpdateContainerReq, actor string) (*Container, error) {
	c, err := s.requireContainer(ctx, id)
	if err != nil {
		return nil, err
	}
	name, err := validateContainerReq(req)
	if err != nil {
		return nil, err
	}
	if existing, err := s.repo.GetContainerByName(ctx, c.ProjectID, name); err == nil && existing.ID != id {
		return nil, NewDevelopmentManagerError(ErrDuplicateContainerName)
	} else if err != nil && !isDMCode(err, ErrContainerNotFound) {
		return nil, err
	}
	replicas := req.Replicas
	if replicas == 0 {
		replicas = 1
	}
	c.ContainerName = name
	c.Image = strings.TrimSpace(req.Image)
	c.ComposeRef = strings.TrimSpace(req.ComposeRef)
	c.Replicas = replicas
	c.NetworkMode = strings.TrimSpace(req.NetworkMode)
	c.EnvNotes = req.EnvNotes
	c.StatusNote = strings.TrimSpace(req.StatusNote)
	c.UpdatedBy = actor
	return s.repo.UpdateContainer(ctx, c)
}

// DeleteContainer 软删除容器（被白名单组引用时拒绝；卷与接入点在事务内级联软删除）
func (s *Service) DeleteContainer(ctx context.Context, id int64) error {
	if _, err := s.requireContainer(ctx, id); err != nil {
		return err
	}
	groups, err := s.repo.WhitelistGroupsOfContainer(ctx, id)
	if err != nil {
		return err
	}
	if len(groups) > 0 {
		return NewDevelopmentManagerError(ErrContainerInWhitelist).WithDetail(
			"被白名单组引用: " + strings.Join(groups, ", ") + "，请先移除成员")
	}
	return s.repo.DeleteContainerTx(ctx, id, time.Now())
}

// ---------- 资源分配 ----------

// GetResourceAlloc 查询容器资源分配
func (s *Service) GetResourceAlloc(ctx context.Context, containerID int64) (*ResourceAlloc, error) {
	if _, err := s.requireContainer(ctx, containerID); err != nil {
		return nil, err
	}
	return s.repo.GetResourceAlloc(ctx, containerID)
}

// UpsertResourceAlloc 写入容器资源分配（1:1）
func (s *Service) UpsertResourceAlloc(ctx context.Context, containerID int64, req *UpsertResourceReq) (*ResourceAlloc, error) {
	if _, err := s.requireContainer(ctx, containerID); err != nil {
		return nil, err
	}
	if req.CPUShares < 0 {
		return nil, NewDevelopmentManagerError(ErrInvalidResourceAlloc)
	}
	return s.repo.UpsertResourceAlloc(ctx, &ResourceAlloc{
		ContainerID:    containerID,
		CPULimit:       strings.TrimSpace(req.CPULimit),
		CPUShares:      req.CPUShares,
		MemLimit:       strings.TrimSpace(req.MemLimit),
		MemReservation: strings.TrimSpace(req.MemReservation),
		GPUNote:        strings.TrimSpace(req.GPUNote),
		StorageQuota:   strings.TrimSpace(req.StorageQuota),
		Notes:          strings.TrimSpace(req.Notes),
	})
}

// ---------- 卷映射 ----------

// ListVolumes 查询容器卷映射
func (s *Service) ListVolumes(ctx context.Context, containerID int64, includeDeleted bool) ([]VolumeMapping, error) {
	if _, err := s.requireContainer(ctx, containerID); err != nil {
		return nil, err
	}
	return s.repo.ListVolumes(ctx, containerID, includeDeleted)
}

// validateVolumeReq 校验卷映射请求
func validateVolumeReq(req *CreateVolumeReq) (string, string, string, error) {
	host := strings.TrimSpace(req.HostPath)
	path := strings.TrimSpace(req.ContainerPath)
	if host == "" || path == "" {
		return "", "", "", NewDevelopmentManagerError(ErrInvalidVolumePath)
	}
	mode := strings.TrimSpace(req.Mode)
	if mode == "" {
		mode = VolumeModeRW
	}
	if !ValidVolumeModes[mode] {
		return "", "", "", NewDevelopmentManagerError(ErrInvalidVolumeMode)
	}
	return host, path, mode, nil
}

// CreateVolume 新建卷映射
func (s *Service) CreateVolume(ctx context.Context, containerID int64, req *CreateVolumeReq) (*VolumeMapping, error) {
	if _, err := s.requireContainer(ctx, containerID); err != nil {
		return nil, err
	}
	host, path, mode, err := validateVolumeReq(req)
	if err != nil {
		return nil, err
	}
	return s.repo.CreateVolume(ctx, &VolumeMapping{
		ContainerID:   containerID,
		HostPath:      host,
		ContainerPath: path,
		Mode:          mode,
		Purpose:       strings.TrimSpace(req.Purpose),
		InBackup:      req.InBackup,
	})
}

// UpdateVolume 更新卷映射
func (s *Service) UpdateVolume(ctx context.Context, id int64, req *UpdateVolumeReq) (*VolumeMapping, error) {
	v, err := s.repo.GetVolume(ctx, id)
	if err != nil {
		return nil, err
	}
	if v.Deleted {
		return nil, NewDevelopmentManagerError(ErrVolumeNotFound)
	}
	host, path, mode, err := validateVolumeReq(req)
	if err != nil {
		return nil, err
	}
	v.HostPath = host
	v.ContainerPath = path
	v.Mode = mode
	v.Purpose = strings.TrimSpace(req.Purpose)
	v.InBackup = req.InBackup
	return s.repo.UpdateVolume(ctx, v)
}

// DeleteVolume 软删除卷映射
func (s *Service) DeleteVolume(ctx context.Context, id int64) error {
	v, err := s.repo.GetVolume(ctx, id)
	if err != nil {
		return err
	}
	if v.Deleted {
		return NewDevelopmentManagerError(ErrVolumeNotFound)
	}
	return s.repo.SoftDeleteVolume(ctx, id, time.Now())
}

// ---------- 网络接入点 ----------

// ListAccessPoints 查询项目网络接入点
func (s *Service) ListAccessPoints(ctx context.Context, projectID int64, req *ListDeploymentReq) ([]AccessPoint, error) {
	if _, err := s.requireProject(ctx, projectID); err != nil {
		return nil, err
	}
	if req.Exposure != "" && !ValidExposures[req.Exposure] {
		return nil, NewDevelopmentManagerError(ErrInvalidExposure)
	}
	if req.Zone != "" && !ValidZones[req.Zone] {
		return nil, NewDevelopmentManagerError(ErrInvalidNetworkZone)
	}
	return s.repo.ListAccessPoints(ctx, projectID, req)
}

// GetAccessPoint 查询接入点详情
func (s *Service) GetAccessPoint(ctx context.Context, id int64) (*AccessPoint, error) {
	return s.repo.GetAccessPoint(ctx, id)
}

// normalizeAccessPoint 校验并归一化接入点请求
func normalizeAccessPoint(req *CreateAccessPointReq) (*AccessPoint, error) {
	if req.ListenPort < 1 || req.ListenPort > 65535 {
		return nil, NewDevelopmentManagerError(ErrInvalidPort)
	}
	if req.HostPort != nil && (*req.HostPort < 1 || *req.HostPort > 65535) {
		return nil, NewDevelopmentManagerError(ErrInvalidPort)
	}
	protocol := req.Protocol
	if protocol == "" {
		protocol = ProtocolTCP
	}
	if !ValidProtocols[protocol] {
		return nil, NewDevelopmentManagerError(ErrInvalidProtocol)
	}
	exposure := req.Exposure
	if exposure == "" {
		exposure = ExposureIntranet
	}
	if !ValidExposures[exposure] {
		return nil, NewDevelopmentManagerError(ErrInvalidExposure)
	}
	proxyType := strings.TrimSpace(req.ProxyType)
	if !ValidProxyTypes[proxyType] {
		return nil, NewDevelopmentManagerError(ErrInvalidProxyType)
	}
	zone := req.NetworkZone
	if zone == "" {
		zone = ZoneIntranet
	}
	if !ValidZones[zone] {
		return nil, NewDevelopmentManagerError(ErrInvalidNetworkZone)
	}
	var url *string
	reachable := false
	if req.URL != nil {
		if trimmed := strings.TrimSpace(*req.URL); trimmed != "" {
			url = &trimmed
			reachable = true
		}
	}
	return &AccessPoint{
		ContainerID: req.ContainerID,
		ListenPort:  req.ListenPort,
		HostPort:    req.HostPort,
		Protocol:    protocol,
		Purpose:     strings.TrimSpace(req.Purpose),
		Exposure:    exposure,
		ProxyType:   proxyType,
		Domain:      strings.TrimSpace(req.Domain),
		UpstreamURL: strings.TrimSpace(req.UpstreamURL),
		TLS:         req.TLS,
		NetworkZone: zone,
		URL:         url,
		Reachable:   reachable,
		Notes:       strings.TrimSpace(req.Notes),
		WhitelistID: req.WhitelistID,
	}, nil
}

// validateAccessPointRefs 校验容器与白名单组归属同一项目
func (s *Service) validateAccessPointRefs(ctx context.Context, projectID int64, ap *AccessPoint) error {
	if ap.ContainerID != nil {
		if _, err := s.requireProjectContainer(ctx, projectID, *ap.ContainerID); err != nil {
			return err
		}
	}
	if ap.WhitelistID != nil {
		w, err := s.repo.GetWhitelist(ctx, *ap.WhitelistID)
		if err != nil {
			return err
		}
		if w.Deleted {
			return NewDevelopmentManagerError(ErrWhitelistNotFound)
		}
		if w.ProjectID != projectID {
			return NewDevelopmentManagerError(ErrCrossProjectRef)
		}
	}
	return nil
}

// CreateAccessPoint 新建接入点
func (s *Service) CreateAccessPoint(ctx context.Context, projectID int64, req *CreateAccessPointReq, actor string) (*AccessPoint, error) {
	if _, err := s.requireProject(ctx, projectID); err != nil {
		return nil, err
	}
	ap, err := normalizeAccessPoint(req)
	if err != nil {
		return nil, err
	}
	if err := s.validateAccessPointRefs(ctx, projectID, ap); err != nil {
		return nil, err
	}
	ap.ProjectID = projectID
	ap.CreatedBy = actor
	ap.UpdatedBy = actor
	return s.repo.CreateAccessPoint(ctx, ap)
}

// UpdateAccessPoint 全量更新接入点
func (s *Service) UpdateAccessPoint(ctx context.Context, id int64, req *UpdateAccessPointReq, actor string) (*AccessPoint, error) {
	existing, err := s.repo.GetAccessPoint(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing.Deleted {
		return nil, NewDevelopmentManagerError(ErrAccessPointNotFound)
	}
	ap, err := normalizeAccessPoint(req)
	if err != nil {
		return nil, err
	}
	if err := s.validateAccessPointRefs(ctx, existing.ProjectID, ap); err != nil {
		return nil, err
	}
	ap.ID = id
	ap.ProjectID = existing.ProjectID
	ap.CreatedBy = existing.CreatedBy
	ap.UpdatedBy = actor
	return s.repo.UpdateAccessPoint(ctx, ap)
}

// DeleteAccessPoint 软删除接入点
func (s *Service) DeleteAccessPoint(ctx context.Context, id int64) error {
	ap, err := s.repo.GetAccessPoint(ctx, id)
	if err != nil {
		return err
	}
	if ap.Deleted {
		return NewDevelopmentManagerError(ErrAccessPointNotFound)
	}
	return s.repo.SoftDeleteAccessPoint(ctx, id, time.Now())
}

// ---------- 白名单 ----------

// ListWhitelists 查询项目白名单组
func (s *Service) ListWhitelists(ctx context.Context, projectID int64, includeDeleted bool) ([]Whitelist, error) {
	if _, err := s.requireProject(ctx, projectID); err != nil {
		return nil, err
	}
	return s.repo.ListWhitelists(ctx, projectID, includeDeleted)
}

// GetWhitelist 查询白名单组详情
func (s *Service) GetWhitelist(ctx context.Context, id int64) (*Whitelist, error) {
	return s.repo.GetWhitelist(ctx, id)
}

// validateWhitelistReq 校验白名单组请求
func validateWhitelistReq(req *CreateWhitelistReq) (string, error) {
	name := strings.TrimSpace(req.Name)
	if l := len([]rune(name)); l < 1 || l > 128 {
		return "", NewDevelopmentManagerError(ErrInvalidWhitelistName)
	}
	return name, nil
}

// CreateWhitelist 新建白名单组（允许 0 成员）
func (s *Service) CreateWhitelist(ctx context.Context, projectID int64, req *CreateWhitelistReq, actor string) (*Whitelist, error) {
	if _, err := s.requireProject(ctx, projectID); err != nil {
		return nil, err
	}
	name, err := validateWhitelistReq(req)
	if err != nil {
		return nil, err
	}
	if _, err := s.repo.GetWhitelistByName(ctx, projectID, name); err == nil {
		return nil, NewDevelopmentManagerError(ErrDuplicateWhitelistName)
	} else if !isDMCode(err, ErrWhitelistNotFound) {
		return nil, err
	}
	return s.repo.CreateWhitelist(ctx, &Whitelist{
		ProjectID: projectID,
		Name:      name,
		Note:      strings.TrimSpace(req.Note),
		CreatedBy: actor,
		UpdatedBy: actor,
	})
}

// UpdateWhitelist 更新白名单组
func (s *Service) UpdateWhitelist(ctx context.Context, id int64, req *UpdateWhitelistReq, actor string) (*Whitelist, error) {
	w, err := s.repo.GetWhitelist(ctx, id)
	if err != nil {
		return nil, err
	}
	if w.Deleted {
		return nil, NewDevelopmentManagerError(ErrWhitelistNotFound)
	}
	name, err := validateWhitelistReq(req)
	if err != nil {
		return nil, err
	}
	if existing, err := s.repo.GetWhitelistByName(ctx, w.ProjectID, name); err == nil && existing.ID != id {
		return nil, NewDevelopmentManagerError(ErrDuplicateWhitelistName)
	} else if err != nil && !isDMCode(err, ErrWhitelistNotFound) {
		return nil, err
	}
	w.Name = name
	w.Note = strings.TrimSpace(req.Note)
	w.UpdatedBy = actor
	return s.repo.UpdateWhitelist(ctx, w)
}

// DeleteWhitelist 删除白名单组（被接入点引用时拒绝）
func (s *Service) DeleteWhitelist(ctx context.Context, id int64) error {
	w, err := s.repo.GetWhitelist(ctx, id)
	if err != nil {
		return err
	}
	if w.Deleted {
		return NewDevelopmentManagerError(ErrWhitelistNotFound)
	}
	refs, err := s.repo.CountAccessPointRefs(ctx, id)
	if err != nil {
		return err
	}
	if refs > 0 {
		return NewDevelopmentManagerError(ErrWhitelistInUse).WithDetail(
			"仍被 " + itoa(refs) + " 条网络接入点引用，请先解除引用")
	}
	return s.repo.DeleteWhitelistTx(ctx, id, time.Now())
}

// ListWhitelistMembers 查询白名单成员（允许空组）
func (s *Service) ListWhitelistMembers(ctx context.Context, whitelistID int64) ([]WhitelistMember, error) {
	w, err := s.repo.GetWhitelist(ctx, whitelistID)
	if err != nil {
		return nil, err
	}
	if w.Deleted {
		return nil, NewDevelopmentManagerError(ErrWhitelistNotFound)
	}
	return s.repo.ListWhitelistMembers(ctx, whitelistID)
}

// AddWhitelistMember 添加白名单成员
func (s *Service) AddWhitelistMember(ctx context.Context, whitelistID int64, req *CreateWhitelistMemberReq) (*WhitelistMember, error) {
	w, err := s.repo.GetWhitelist(ctx, whitelistID)
	if err != nil {
		return nil, err
	}
	if w.Deleted {
		return nil, NewDevelopmentManagerError(ErrWhitelistNotFound)
	}
	if _, err := s.requireProjectContainer(ctx, w.ProjectID, req.ContainerID); err != nil {
		return nil, err
	}
	exists, err := s.repo.WhitelistMemberExists(ctx, whitelistID, req.ContainerID)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, NewDevelopmentManagerError(ErrContainerInWhitelist).WithDetail("该容器已是白名单成员")
	}
	return s.repo.CreateWhitelistMember(ctx, &WhitelistMember{
		WhitelistID: whitelistID,
		ContainerID: req.ContainerID,
		Note:        strings.TrimSpace(req.Note),
	})
}

// RemoveWhitelistMember 移除白名单成员
func (s *Service) RemoveWhitelistMember(ctx context.Context, whitelistID, memberID int64) error {
	m, err := s.repo.GetWhitelistMember(ctx, memberID)
	if err != nil {
		return err
	}
	if m.WhitelistID != whitelistID {
		return NewDevelopmentManagerError(ErrWhitelistMemberNotFound)
	}
	return s.repo.DeleteWhitelistMember(ctx, memberID)
}

// ---------- 备份 ----------

// ListBackups 查询项目备份记录
func (s *Service) ListBackups(ctx context.Context, projectID int64) ([]Backup, error) {
	if _, err := s.requireProject(ctx, projectID); err != nil {
		return nil, err
	}
	return s.repo.ListBackups(ctx, projectID)
}

// GetBackup 查询备份记录详情
func (s *Service) GetBackup(ctx context.Context, id int64) (*Backup, error) {
	return s.repo.GetBackup(ctx, id)
}

// validateBackupReq 校验备份请求
func validateBackupReq(req *CreateBackupReq) error {
	if !ValidBackupTypes[req.BackupType] {
		return NewDevelopmentManagerError(ErrInvalidBackupType)
	}
	if req.RetentionDays < 0 {
		return NewDevelopmentManagerError(ErrInvalidBackupType)
	}
	return nil
}

// validateBackupRefs 校验备份关联的容器/卷属于同一项目
func (s *Service) validateBackupRefs(ctx context.Context, projectID int64, containerID, volumeID *int64) error {
	if containerID != nil {
		if _, err := s.requireProjectContainer(ctx, projectID, *containerID); err != nil {
			return err
		}
	}
	if volumeID != nil {
		v, err := s.repo.GetVolume(ctx, *volumeID)
		if err != nil {
			return err
		}
		if v.Deleted {
			return NewDevelopmentManagerError(ErrVolumeNotFound)
		}
		if _, err := s.requireProjectContainer(ctx, projectID, v.ContainerID); err != nil {
			return err
		}
	}
	return nil
}

// CreateBackup 新建备份记录
func (s *Service) CreateBackup(ctx context.Context, projectID int64, req *CreateBackupReq) (*Backup, error) {
	if _, err := s.requireProject(ctx, projectID); err != nil {
		return nil, err
	}
	if err := validateBackupReq(req); err != nil {
		return nil, err
	}
	if err := s.validateBackupRefs(ctx, projectID, req.ContainerID, req.VolumeID); err != nil {
		return nil, err
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	return s.repo.CreateBackup(ctx, &Backup{
		ProjectID:     projectID,
		ContainerID:   req.ContainerID,
		VolumeID:      req.VolumeID,
		BackupType:    req.BackupType,
		StrategyCron:  strings.TrimSpace(req.StrategyCron),
		Destination:   strings.TrimSpace(req.Destination),
		RetentionDays: req.RetentionDays,
		RestoreNote:   strings.TrimSpace(req.RestoreNote),
		Enabled:       enabled,
	})
}

// UpdateBackup 更新备份记录（含最近执行结果回报）
func (s *Service) UpdateBackup(ctx context.Context, id int64, req *UpdateBackupReq) (*Backup, error) {
	b, err := s.repo.GetBackup(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := validateBackupReq(&req.CreateBackupReq); err != nil {
		return nil, err
	}
	if !ValidBackupStatuses[req.LastStatus] {
		return nil, NewDevelopmentManagerError(ErrInvalidBackupStatus)
	}
	if err := s.validateBackupRefs(ctx, b.ProjectID, req.ContainerID, req.VolumeID); err != nil {
		return nil, err
	}
	b.ContainerID = req.ContainerID
	b.VolumeID = req.VolumeID
	b.BackupType = req.BackupType
	b.StrategyCron = strings.TrimSpace(req.StrategyCron)
	b.Destination = strings.TrimSpace(req.Destination)
	b.RetentionDays = req.RetentionDays
	b.RestoreNote = strings.TrimSpace(req.RestoreNote)
	if req.Enabled != nil {
		b.Enabled = *req.Enabled
	}
	b.LastStatus = req.LastStatus
	b.LastError = strings.TrimSpace(req.LastError)
	b.LastRunAt = req.LastRunAt
	return s.repo.UpdateBackup(ctx, b)
}

// DeleteBackup 删除备份记录
func (s *Service) DeleteBackup(ctx context.Context, id int64) error {
	if _, err := s.repo.GetBackup(ctx, id); err != nil {
		return err
	}
	return s.repo.DeleteBackup(ctx, id)
}

// itoa 轻量整数转字符串（避免为单处引入 strconv）
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
