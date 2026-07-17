## 变更说明

<!-- 用户可见的行为变化、关联 issue、迁移或配置变更 -->

## 验证

<!-- 执行过的命令与结果;UI 变更附截图 -->

## 检查清单

- [ ] `make check`、`go test -race ./...`、`make build` 全部通过
- [ ] 接口契约变更已同步 `api/openapi.yaml` 并通过 `make test-contract`
- [ ] UI 变更附截图,并完成加载 / 空态 / 错误 / 移动端 / 键盘 / 暗色主题冒烟
- [ ] Bug 修复附带回归测试
- [ ] 数据库变更只在 `migrations/` 追加新迁移文件
- [ ] 提交信息符合 Conventional Commits(英文主题)
