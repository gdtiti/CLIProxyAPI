// Package models 定义多用户模块的数据模型
//
// 本包包含以下核心模型：
//   - User: 用户模型，包含身份认证和授权信息
//   - UserGroup: 用户组模型，支持层级结构和共享配置
//   - OAuthAccount: 第三方登录账号模型
//
// 以及相关的枚举类型：
//   - UserRole: 用户角色（admin、operator、user）
//   - UserStatus: 用户状态（active、suspended、disabled、pending）
//   - RegisterSource: 注册来源（local、google、wechat、github、invite、admin）
//   - OAuthProvider: OAuth 供应商（google、wechat、github、apple、microsoft）
package models
