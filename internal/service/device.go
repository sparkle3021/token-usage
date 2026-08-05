package service

// Package service 的子文件，设备注册表业务逻辑（设备列表 / 重命名展示名）。
import (
	"fmt"
	"log"

	"token-dashboard/internal/database"
	"token-dashboard/internal/model"
)

// DeviceService 设备注册表业务逻辑。
type DeviceService struct {
	db *database.Manager
}

// NewDeviceService 构建设备服务实例。
func NewDeviceService(db *database.Manager) *DeviceService {
	return &DeviceService{db: db}
}

// ListDevices 返回全部设备信息。
func (s *DeviceService) ListDevices() []model.DeviceInfo {
	if s.db == nil {
		return nil
	}
	devices, err := s.db.ListDevices()
	if err != nil {
		log.Printf("[service] ListDevices error: %v", err)
		return nil
	}
	return devices
}

// RenameDevice 修改设备展示名，仅作用于 devices.display_name。
func (s *DeviceService) RenameDevice(deviceID, displayName string) error {
	if s.db == nil {
		return fmt.Errorf("数据库未初始化")
	}
	if deviceID == "" {
		return fmt.Errorf("设备 id 不能为空")
	}
	return s.db.RenameDevice(deviceID, displayName)
}
