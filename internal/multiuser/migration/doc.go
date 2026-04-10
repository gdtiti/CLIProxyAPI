// Package migration 提供多用户模块的数据库迁移功能
//
// 本包实现了以下功能：
//   - AutoMigrate: 自动创建和更新数据表结构
//   - CreateIndexes: 创建数据库索引（包括唯一约束）
//   - CreateDefaultAdmin: 创建默认管理员账号
//
// 迁移策略：
//   - 使用 GORM AutoMigrate 进行表结构同步
//   - 手动创建复合唯一索引
//   - 首次运行时创建默认管理员
package migration
