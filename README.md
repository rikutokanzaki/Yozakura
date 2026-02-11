# Yozakura - 静的多層型ハニーポットシステム

Yozakuraは、Sakuraの静的構成版として設計された多層型ハニーポットシステムです。Launcherを持たず、全てのハニーポットが常時起動する構成です。

## 概要

YozakuraはSakuraから動的制御機能（Launcher）を除き、以下の2層構造で構成されています：

### 1. Dispatcher

- **Paramiko（SSH）**: SSH接続を受け付け、認証・コマンド実行を監視してハニーポットへルーティング
- **OpenResty（HTTP）**: リクエストを静的に振り分け（動的起動なし）

### 2. Layers

- **Active Layer（能動型）**
  - **Heralding**: SSH/HTTPなど複数プロトコルの認証試行を記録
- **Passive Layer（受動型）**
  - **Cowrie**: SSH MITMハニーポット
  - **Wordpot**: WordPress特化型ハニーポット
  - **H0neytr4p**: Web攻撃全般を記録する

### 3. ELK Stack

- **Elasticsearch**: ログデータの保存・検索
- **Logstash**: 各ハニーポットからのログを正規化・集約
- **Kibana**: ダッシュボードによる可視化

## アーキテクチャ

```
攻撃者
  ↓
┌─────────────────────────────────────┐
│ Dispatcher                          │
│  - Paramiko (SSH:22)                │
│  - OpenResty (HTTP:80)              │
└─────────────────────────────────────┘
  ↓ 振り分け（常時起動）
┌──────────────────────────────────────┐
│ Honeypots                            │
│  Active:  Heralding                  │
│  Passive: Cowrie, Wordpot, H0neytr4p │
└──────────────────────────────────────┘
  ↓ ログ出力
┌──────────────────────────────────────┐
│ ELK Stack                            │
│  - Elasticsearch                     │
│  - Logstash                          │
│  - Kibana                            │
└──────────────────────────────────────┘
```

## 主な特徴

### Sakuraとの違い

- **Launcherなし**: 動的制御機能を削除し、静的構成に
- **常時起動**: 全てのハニーポットが起動状態を維持
- **静的振り分け**: Luaスクリプトによるパターンマッチングのみ
- **軽量化**: リソース管理機能を省略し、運用を簡素化

### SSH処理

- Paramiko Dispatcherで認証情報を記録後、直接Cowrieへ転送
- セッション移行なし（常にCowrieで処理）

### HTTP処理

- OpenRestyのLuaスクリプトでパターンマッチング
- WordPressパターン → Wordpot
- 攻撃ツール検知 → H0neytr4p
- 通常アクセス → Heralding

## 構成ファイル

### ディレクトリ構造

```
Yozakura/
├── install.sh               # インストールスクリプト
├── uninstall.sh             # アンインストール・バックアップスクリプト
├── .env                     # 環境変数設定
├── compose/                 # Docker Composeプロファイル
│     ├── standard.yml       # 全機能有効
│     ├── ssh.yml            # SSHのみ
│     └── http.yml           # HTTPのみ
├── dispatcher/
│     ├── paramiko/          # SSHリバースプロキシ
│     │     ├── Dockerfile
│     │     ├── requirements.txt
│     │     ├── config/
│     │     │     ├── user.txt
│     │     │     └── motd.txt
│     │     └── src/
│     │           ├── main.py
│     │           ├── auth/
│     │           ├── connector/
│     │           ├── session/
│     │           ├── detector/
│     │           ├── reader/
│     │           └── utils/
│     └── openresty/         # HTTPリバースプロキシ
│           ├── Dockerfile
│           ├── nginx.conf
│           ├── conf.d/
│           │     └── http.conf
│           └── lua/
│                 └── detect.lua    # 振り分けロジック
├── layers/
│     └── core/
│           └── config/
│                 └── userdb.txt    # Cowrie認証
├── elk/
│     ├── logstash/
│     │     └── logstash.conf
│     ├── kibana/
│     │     └── export.ndjson
│     └── metricbeat/
│           └── metricbeat.yml
└── data/                           # 各ハニーポットのログ出力先
      ├── paramiko/
      ├── openresty/
      ├── heralding/
      ├── cowrie/
      ├── wordpot/
      └── h0neytr4p/
```

## インストール

### 前提条件

- Docker & Docker Compose
- sudo権限
- ポート22, 80が利用可能

### 手順

#### 1. **環境変数設定**

`.env`ファイルを編集：

```bash
####################################
### Server Configuration
####################################
YOZAKURA_DATA_PATH=../data

ARCHIVE_DATA_PATH=/path/to/archive

ALLOWED_NETWORKS=

HOST_NAME=svr04

####################################
### Elastic Stack
####################################

# Project namespace (defaults to the current folder name if not set)
#COMPOSE_PROJECT_NAME=myproject


# Password for the 'elastic' user (at least 6 characters)
ELASTIC_PASSWORD=


# Password for the 'kibana_system' user (at least 6 characters)
KIBANA_PASSWORD=


# Version of Elastic products
STACK_VERSION=8.7.1


# Set the cluster name
CLUSTER_NAME=docker-cluster


# Set to 'basic' or 'trial' to automatically start the 30-day trial
LICENSE=basic
#LICENSE=trial


# Increase or decrease based on the available host memory (in bytes)
ES_MEM_LIMIT=1073741824
KB_MEM_LIMIT=1073741824
LS_MEM_LIMIT=1073741824


# SAMPLE Predefined Key only to be used in POC environments
ENCRYPTION_KEY=

```

#### 2. **インストール実行**

```bash
./install.sh
```

#### 3. **プロファイル選択**

インストールスクリプトが起動時に以下から選択を求めます：

- `standard.yml`: SSH + HTTP（推奨）
- `ssh.yml`: SSH のみ
- `http.yml`: HTTP のみ

#### 4. **動作確認**

```bash
docker compose -f compose/standard.yml ps
```

## 使用方法

### Kibanaダッシュボード

```
http://localhost:64297
ユーザー名: elastic
パスワード: (KIBANA_PASSWORD)
```

## アンインストール

```bash
./uninstall.sh
```

アンインストール時、`data/`ディレクトリは自動的にバックアップされます：

```
${ARCHIVE_DATA_PATH}/Yozakura/${INSTALL_DATE}-${TODAY}-${TIME}/data/
```

## 設定カスタマイズ

### WordPressパターンの変更

[dispatcher/openresty/lua/detect.lua](dispatcher/openresty/lua/detect.lua) の `wordpress_patterns` 配列を編集

### Cowrieユーザーアカウントの追加

[layers/core/config/userdb.txt](layers/core/config/userdb.txt) を編集（Cowrie標準形式）

## 技術詳細

### Paramiko Dispatcher

- Paramiko ServerInterfaceによるSSHサーバー実装
- Launcherへの通知機能なし
- Cowrieへの直接転送（常時接続）

### OpenResty Dispatcher

- Luaスクリプトによる静的パターンマッチング
- 動的起動機能なし
- シンプルなproxy_pass処理

### Logstash

- 各ハニーポットの異なるログフォーマットを統一
- CSV（Heralding）、JSON（Cowrie/Wordpot/H0neytr4p）、カスタム（NGINX）を正規化
- `src_ip`, `src_port`, `dest_port`, `username`, `password`, `request_uri` などの共通フィールドへマッピング

## 関連プロジェクト

- **Sakura**: 動的多層型ハニーポットシステム
- **Spring**: ハニーポットシステム切替運用フレームワーク
- **Tsubomi**: ハニーポット単体運用
- **bloom-insight**: ログ分析・評価システム
