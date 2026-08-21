// Package xpersistence 提供 demo-dog 的持久化抽象。
//
// 历史上 demo-dog 的配置类状态(tenants / admin keys / OIDC provider /
// retention policies / circuit breakers / rate limit buckets / quota /
// alert rules / webhook subscribers / audit log)都是 in-memory only,
// 进程重启即丢。档 B 多租户 SaaS 化要求所有这些状态在
// 重启后保留,所以 W1 引入一个轻量 KV 抽象。
//
// 设计原则:
//   - 接口稳定,具体后端可替换;默认 filejson 适合单进程自托管,
//     生产可换 bbolt / sqlite / postgres。
//   - 写入保证 fsync,不会在 crash 后回滚。
//   - 读路径无锁;写路径持进程内 RWMutex + 跨进程文件锁。
//   - 零外部依赖:本包默认后端仅依赖 stdlib + sync。
//
// 包的 KV 用法:
//
//	kv, err := xpersistence.OpenFileJSON(dataDir + "/state.json")
//	if err != nil { return err }
//	if err := kv.Set(ctx, "tenant:acme", []byte(`{...}`)); err != nil { ... }
//	val, err := kv.Get(ctx, "tenant:acme")
//	if errors.Is(err, xpersistence.ErrNotFound) { ... }
//	keys, err := kv.List(ctx, "tenant:")
//
// 复杂的 list 场景(范围查询)由 List(prefix) 提供;不暴露
// Range/Scan API 以保持后端实现简单。
//
// 所有 Set 必须原子的 fsync 落盘;对于必须跨多条 KV 原子写入
// 的场景(如 "create key + update index"),用 WithAtomic:
//
//	err := kv.WithAtomic(ctx, func(tx Tx) error {
//	tx.Set("key:abc", v1)
//	tx.Set("idx:label:abc", v2)
//	return nil
//
// 持久化层暴露的所有错误都通过 errors.Is(err, ErrNotFound /
// ErrCorrupted / ErrClosed) 与调用方沟通。
package xpersistence
