// Package repository 提供多用户模块的数据访问层
//
// 本包实现了以下 Repository 接口：
//   - UserRepository: 用户数据访问，包含 CRUD 和查询操作
//   - GroupRepository: 用户组数据访问，包含层级结构管理
//   - OAuthAccountRepository: 第三方账号数据访问
//
// 所有 Repository 实现都基于 GORM，支持：
//   - 分页查询
//   - 条件筛选
//   - 排序
//   - 事务支持
package repository
