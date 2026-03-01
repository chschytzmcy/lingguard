# LingGuard 部署到 Firefly 设备

将 LingGuard 一键部署到 Firefly 目标设备（RK3588）。

## 关键词

- lingguard部署到firefly设备上
- 部署到firefly
- 部署lingguard

## 目标设备信息

- IP: 192.168.1.103
- 用户名: firefly
- 密码: firefly
- 平台: linux-arm64 (RK3588)
- 服务端口: 18989

## 使用方法

### 快速部署

```
lingguard部署到firefly设备上
```

或:
```
部署到firefly
```

系统自动执行:
1. 构建 Linux arm64 安装包
2. 上传到 192.168.1.103
3. 执行安装脚本
4. 启动服务并验证状态

### 查看状态

```
查看 firefly 设备状态
```

### 重启服务

```
重启 firefly 服务
```

## 部署流程

1. **构建** - `make package-linux-arm64`
2. **上传** - scp 到 192.168.1.103:/tmp
3. **解压** - tar -xzf 解压
4. **安装** - scripts/install.sh
5. **启动** - systemd 服务
6. **验证** - 检查状态

## 常用命令

```bash
# 本地构建
make clean
make package-linux-arm64

# SSH 连接
ssh firefly@192.168.1.103

# 服务管理
systemctl --user status lingguard
systemctl --user restart lingguard
journalctl --user -u lingguard -f

# 应用日志
tail -f ~/.lingguard/logs/lingguard.log

# Web UI
curl http://127.0.0.1:18989/
```

## 故障排除

```bash
# 测试 SSH
ssh firefly@192.168.1.103 "echo OK"

# 检查端口
ssh firefly@192.168.1.103 "ss -tlnp | grep 18989"

# 查看错误
ssh firefly@192.168.1.103 "journalctl --user -u lingguard -n 50"
```