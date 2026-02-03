// Package config 提供多用户模块的配置管理
//
// 本包定义了以下配置结构：
//   - MultiUserConfig: 多用户模块主配置
//   - DatabaseConfig: 数据库连接配置（DSN、schema、连接池）
//   - AuthConfig: 认证配置（JWT 密钥、Token 过期时间）
//   - RegistrationConfig: 注册配置（是否允许注册、默认余额、默认角色）
//   - AdminConfig: 管理员配置（默认管理员用户名、邮箱、密码）
//
// 配置支持：
//   - 从配置文件加载
//   - 环境变量覆盖
//   - 默认值设置
package config
