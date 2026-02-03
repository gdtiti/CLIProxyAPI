// Package multiuser 提供 CLI Proxy API 的多用户管理功能
//
// 本模块实现了完整的多用户管理系统，包括：
//   - 用户管理：CRUD、状态管理、角色管理
//   - 用户组管理：层级结构、成员管理、共享配置
//   - 认证服务：本地登录、JWT Token、第三方登录
//   - RESTful API：完整的管理和用户接口
//
// 技术栈：
//   - ORM: GORM
//   - 数据库: PostgreSQL
//   - Web 框架: Gin
//   - 认证: JWT
//   - 文档: Swagger
//
// 目录结构：
//   - models/: 数据模型
//   - repository/: 数据访问层
//   - service/: 业务逻辑层
//   - api/: HTTP API 层
//   - config/: 配置管理
//   - migration/: 数据库迁移
//
// 使用示例：
//
//	import "github.com/user/project/internal/multiuser"
//
//	// 初始化模块
//	err := multiuser.Initialize(cfg)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer multiuser.Shutdown()
//
//	// 注册路由
//	multiuser.SetupRoutes(router)
package multiuser
