/**
 * 设备展示名解析：device_id → display_name（缺省 hostname）。
 * deviceNames 由后端附带（DashboardData/TimeSeriesData）或 GetDevices() 组装。
 */

/** 解析单个设备的展示名；无映射时回退 deviceId 原文。 */
export function deviceName(deviceNames, deviceId) {
  if (!deviceId) return '';
  return (deviceNames && deviceNames[deviceId]) || deviceId;
}

/** 从 GetDevices 列表组装 device_id → 展示名 映射（display_name 缺省 hostname）。 */
export function deviceNamesFromList(list) {
  const m = {};
  for (const d of list || []) {
    m[d.deviceId] = d.displayName || d.hostname || d.deviceId;
  }
  return m;
}
