package developmentmanager

import "testing"

func TestNormalizeAccessPoint(t *testing.T) {
	t.Run("缺省值归一化", func(t *testing.T) {
		ap, err := normalizeAccessPoint(&CreateAccessPointReq{ListenPort: 8080})
		if err != nil {
			t.Fatalf("期望成功，实际 %v", err)
		}
		if ap.Protocol != ProtocolTCP || ap.Exposure != ExposureIntranet || ap.NetworkZone != ZoneIntranet {
			t.Errorf("缺省值异常：protocol=%s exposure=%s zone=%s", ap.Protocol, ap.Exposure, ap.NetworkZone)
		}
		if ap.URL != nil || ap.Reachable {
			t.Error("未提供 URL 时应为不可达（url=NULL, reachable=false）")
		}
	})

	t.Run("空字符串 URL 视为不可达", func(t *testing.T) {
		empty := "  "
		ap, err := normalizeAccessPoint(&CreateAccessPointReq{ListenPort: 80, URL: &empty})
		if err != nil {
			t.Fatalf("期望成功，实际 %v", err)
		}
		if ap.URL != nil || ap.Reachable {
			t.Error("空白 URL 应归一化为 NULL 且不可达")
		}
	})

	t.Run("有效 URL 可达", func(t *testing.T) {
		u := " https://api.example.com "
		ap, err := normalizeAccessPoint(&CreateAccessPointReq{ListenPort: 443, URL: &u})
		if err != nil {
			t.Fatalf("期望成功，实际 %v", err)
		}
		if ap.URL == nil || *ap.URL != "https://api.example.com" || !ap.Reachable {
			t.Errorf("URL 归一化异常：%v / reachable=%v", ap.URL, ap.Reachable)
		}
	})

	t.Run("端口非法", func(t *testing.T) {
		for _, port := range []int{0, -1, 65536} {
			if _, err := normalizeAccessPoint(&CreateAccessPointReq{ListenPort: port}); err == nil {
				t.Errorf("端口 %d 应校验失败", port)
			}
		}
		bad := 70000
		if _, err := normalizeAccessPoint(&CreateAccessPointReq{ListenPort: 80, HostPort: &bad}); err == nil {
			t.Error("host_port 70000 应校验失败")
		}
	})

	t.Run("枚举非法", func(t *testing.T) {
		cases := []*CreateAccessPointReq{
			{ListenPort: 80, Protocol: "http"},
			{ListenPort: 80, Exposure: "public"},
			{ListenPort: 80, NetworkZone: "dmz"},
			{ListenPort: 80, ProxyType: "apache"},
		}
		for i, req := range cases {
			if _, err := normalizeAccessPoint(req); err == nil {
				t.Errorf("用例 %d 应校验失败", i)
			}
		}
	})

	t.Run("反代留空合法（直连）", func(t *testing.T) {
		if _, err := normalizeAccessPoint(&CreateAccessPointReq{ListenPort: 80, ProxyType: ""}); err != nil {
			t.Errorf("空反代类型应合法，实际 %v", err)
		}
	})
}

func TestValidateContainerReq(t *testing.T) {
	if _, err := validateContainerReq(&CreateContainerReq{ContainerName: " "}); err == nil {
		t.Error("空容器名应校验失败")
	}
	if _, err := validateContainerReq(&CreateContainerReq{ContainerName: "api", Replicas: -1}); err == nil {
		t.Error("负副本数应校验失败")
	}
	name, err := validateContainerReq(&CreateContainerReq{ContainerName: " api "})
	if err != nil || name != "api" {
		t.Errorf("容器名应 trim，实际 %q / %v", name, err)
	}
}

func TestValidateVolumeReq(t *testing.T) {
	if _, _, _, err := validateVolumeReq(&CreateVolumeReq{HostPath: "", ContainerPath: "/data"}); err == nil {
		t.Error("空宿主路径应校验失败")
	}
	if _, _, _, err := validateVolumeReq(&CreateVolumeReq{HostPath: "/srv", ContainerPath: ""}); err == nil {
		t.Error("空容器路径应校验失败")
	}
	_, _, mode, err := validateVolumeReq(&CreateVolumeReq{HostPath: "/srv", ContainerPath: "/data"})
	if err != nil || mode != VolumeModeRW {
		t.Errorf("挂载模式缺省应为 rw，实际 %s / %v", mode, err)
	}
	if _, _, _, err := validateVolumeReq(&CreateVolumeReq{HostPath: "/srv", ContainerPath: "/data", Mode: "rwx"}); err == nil {
		t.Error("非法挂载模式应校验失败")
	}
}

func TestValidateWhitelistReq(t *testing.T) {
	if _, err := validateWhitelistReq(&CreateWhitelistReq{Name: ""}); err == nil {
		t.Error("空白名单组名应校验失败")
	}
	name, err := validateWhitelistReq(&CreateWhitelistReq{Name: " ia-market-allow "})
	if err != nil || name != "ia-market-allow" {
		t.Errorf("组名应 trim，实际 %q / %v", name, err)
	}
}

func TestValidateBackupReq(t *testing.T) {
	for _, typ := range []string{"", "incremental"} {
		if err := validateBackupReq(&CreateBackupReq{BackupType: typ}); err == nil {
			t.Errorf("备份类型 %q 应校验失败", typ)
		}
	}
	if err := validateBackupReq(&CreateBackupReq{BackupType: BackupTypeDB, RetentionDays: -1}); err == nil {
		t.Error("负保留天数应校验失败")
	}
	if err := validateBackupReq(&CreateBackupReq{BackupType: BackupTypeVolume}); err != nil {
		t.Errorf("合法备份类型应通过，实际 %v", err)
	}
}

func TestDeploymentCollections(t *testing.T) {
	if len(ValidProtocols) != 2 {
		t.Errorf("协议应为 2 种，实际 %d", len(ValidProtocols))
	}
	if len(ValidExposures) != 3 {
		t.Errorf("暴露范围应为 3 种，实际 %d", len(ValidExposures))
	}
	if len(ValidZones) != 3 {
		t.Errorf("网络区域应为 3 种，实际 %d", len(ValidZones))
	}
	if len(ValidVolumeModes) != 2 {
		t.Errorf("挂载模式应为 2 种，实际 %d", len(ValidVolumeModes))
	}
	if len(ValidBackupTypes) != 3 {
		t.Errorf("备份类型应为 3 种，实际 %d", len(ValidBackupTypes))
	}
	if !ValidProxyTypes[""] || !ValidProxyTypes[ProxyNginx] {
		t.Error("反代类型应允许留空与 nginx")
	}
	if !ValidBackupStatuses[""] || !ValidBackupStatuses[BackupStatusRunning] {
		t.Error("备份状态应允许留空与 running")
	}
}

func TestDeploymentStatusForCode(t *testing.T) {
	cases := map[int]int{
		ErrContainerNotFound:      404,
		ErrWhitelistNotFound:      404,
		ErrBackupNotFound:         404,
		ErrDuplicateContainerName: 409,
		ErrContainerInWhitelist:   409,
		ErrWhitelistInUse:         409,
		ErrInvalidPort:            400,
		ErrInvalidProxyType:       400,
		ErrCrossProjectRef:        400,
	}
	for code, want := range cases {
		if got := statusForCode(code); got != want {
			t.Errorf("code %d 期望状态 %d，实际 %d", code, want, got)
		}
	}
}

func TestItoa(t *testing.T) {
	cases := map[int]string{0: "0", 1: "1", 42: "42", 12345: "12345"}
	for n, want := range cases {
		if got := itoa(n); got != want {
			t.Errorf("itoa(%d)=%s，期望 %s", n, got, want)
		}
	}
}
