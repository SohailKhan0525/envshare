# 🔐 Envshare

> **Zero-Trust Team Secret Sharing CLI & Server**  
> Share passwords, `.env` files, and secrets safely across your team without ever pasting them into chat/email, and without the server ever seeing unencrypted secrets.

[![npm version](https://img.shields.io/npm/v/@agent-qofeno/envshare?color=blue&logo=npm)](https://www.npmjs.com/package/@agent-qofeno/envshare)
[![npm downloads](https://img.shields.io/npm/dm/@agent-qofeno/envshare?logo=npm)](https://www.npmjs.com/package/@agent-qofeno/envshare)
[![GitHub Release](https://img.shields.io/github/v/release/SohailKhan0525/envshare?color=green&logo=github)](https://github.com/SohailKhan0525/envshare/releases)
[![Build Status](https://img.shields.io/github/actions/workflow/status/SohailKhan0525/envshare/release.yml?branch=main&label=build&logo=github)](https://github.com/SohailKhan0525/envshare/actions)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)](https://go.dev)
[![Node.js Version](https://img.shields.io/badge/Node.js-14+-339933?logo=nodedotjs)](https://nodejs.org)

---

## 🌟 Features

- 🔑 **End-to-End Encryption**: Locked on your machine using `age` (X25519) before transmission.
- 🚫 **Zero-Trust Server**: Server only stores scrambled keys and locked ciphertexts.
- ⏳ **Secret Expiration**: Set automatic expiration for secrets (e.g. `envshare push .env staging 30` for 30-day life).
- 📜 **Audit History**: Track who pushed, pulled, or modified team secrets.
- 👥 **Team Access Control**: Seamlessly add or remove team members.
- 💻 **Cross-Platform**: Standalone binaries for Linux, macOS (Intel/Apple Silicon), and Windows.

---

## 🚀 Quick Start & Installation

### Option 1: Via npm (Recommended)

```bash
# Global installation
npm install -g @agent-qofeno/envshare

# Or run directly without installation
npx @agent-qofeno/envshare --help
```

### Option 2: Pre-compiled Binaries
Download ready-to-run binaries from the [Latest GitHub Release](https://github.com/SohailKhan0525/envshare/releases/latest) for Linux, macOS, or Windows.

### Option 3: Build from Source
```bash
go mod tidy
go build -o bin/envshare ./cmd/envshare
go build -o bin/envshare-server ./cmd/envshare-server
```

---

## 🛠️ Everyday CLI Commands

| Command | Description |
| :--- | :--- |
| `envshare keygen` | 🔑 Create your personal private key (run once) |
| `envshare configure` | ⚙️ Configure server address, team name, and personal access code |
| `envshare addmember` | 👤 *(Admin)* Add a new member to the team |
| `envshare removemember` | ❌ *(Admin)* Revoke a member's access from future pushes |
| `envshare push .env staging` | 📤 Encrypt and push `.env` to the `staging` environment |
| `envshare push .env staging 30` | ⏳ Push secret with automatic expiration after 30 days |
| `envshare pull staging .env` | 📥 Fetch and decrypt `staging` secret into `.env` |
| `envshare members` | 👥 List all current team members and public keys |
| `envshare environments` | 🌐 List all shared environments in the team |
| `envshare history` | 📜 View plain audit log of all team secret activity |

---

## 🖥️ Server Setup Guide

Run the single self-contained server binary on any small cloud instance or container:

```bash
EnvshareAdminToken="your-secure-admin-password" \
EnvshareDataDir="./data" \
EnvshareAddr=":8443" \
./bin/envshare-server
```

> 💡 **Tip**: Put `envshare-server` behind an HTTPS proxy (such as Caddy, Nginx, or Railway/Fly.io) in production.

---

## 🛡️ Security Model

Envshare uses the `filippo.io/age` modern encryption library. 
- Every file pushed is encrypted locally using the recipient public keys of all current team members.
- The server never possesses private keys and cannot decrypt your environment files.
- Member removal revokes future distribution; for complete revocation of historical secrets, re-push new environment credentials after removing a member.

---

## 📄 License

This project is licensed under the [MIT License](LICENSE).
