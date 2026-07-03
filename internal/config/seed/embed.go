// Package seed 嵌入默认定价数据，首次运行时自动写入配置目录。
// 确保分发后的应用无需外部文件即可拥有初始定价数据。
package seed

import _ "embed"

//go:embed pricing-litellm.json
var PricingLitellm []byte

//go:embed pricing-openrouter.json
var PricingOpenRouter []byte
