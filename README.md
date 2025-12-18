# Hackz-Ptera Backend

「意図的に不便なUX」をコンセプトにしたハッカソンプロジェクトのバックエンドです。

## 技術スタック

- **Go 1.24** - プログラミング言語
- **Echo v4** - Webフレームワーク
- **gorilla/websocket** - WebSocket通信
- **AWS Bedrock** - Claude 3 Haiku (AI生成)

---

## 環境構築

### ローカル開発 (docker-compose)

プロジェクトルートで以下を実行：

```bash
cd /path/to/hackz-ptera
docker-compose up
```

- バックエンド: http://localhost:8080
- フロントエンド: http://localhost:5173

### 単体で起動する場合

```bash
# 依存関係をインストール
go mod download

# 開発サーバーを起動 (ホットリロード)
air

# または通常起動
go run ./cmd/server
```

---

## 環境変数

`.env.example` を `.env` にコピーして設定：

```env
# Server
PORT=8080
CORS_ORIGIN=http://localhost:5173

# AWS
AWS_REGION=ap-northeast-1
AWS_ACCESS_KEY_ID=
AWS_SECRET_ACCESS_KEY=

# S3
S3_BUCKET=hackz-ptera-assets

# Bedrock
BEDROCK_MODEL_ID=anthropic.claude-3-haiku-20240307-v1:0
```

---

## CI/CD

### ブランチ戦略

| ブランチ | 環境 | 動作 |
|---------|------|------|
| `main` | 本番 | AWS ECS に自動デプロイ |
| `develop` | 開発 | CI (テスト・Lint) のみ |
| `feat/*` | 機能開発 | developから作成 |
| `fix/*` | バグ修正 | developから作成 |

#### ブランチ運用ルール

```
main (本番環境)
  ↑ マージ (PR必須)
develop (開発・統合)
  ↑ マージ (PR必須)
feat/xxx または fix/xxx (作業ブランチ)
```

1. **機能開発時**: `develop` から `feat/機能名` ブランチを作成
2. **バグ修正時**: `develop` から `fix/バグ名` ブランチを作成
3. **作業完了後**: `develop` へPRを作成してマージ
4. **リリース時**: `develop` から `main` へPRを作成してマージ → 自動デプロイ

#### コマンド例

```bash
# 新機能開発を始める
git checkout develop
git pull origin develop
git checkout -b feat/add-new-api

# 作業完了後
git add .
git commit -m "feat: 新しいAPIを追加"
git push -u origin feat/add-new-api
# → GitHub でPRを作成

# developへマージ後、本番リリース
git checkout develop
git pull origin develop
git checkout main
git pull origin main
git merge develop
git push origin main
# → 自動でECSにデプロイ
```

### GitHub Actions

- **CI** (`ci.yml`): push/PR時にテスト・Lint・ビルド確認
- **CD** (`deploy.yml`): mainブランチへのpush時にECSへデプロイ

### 本番環境

| リソース | 値 |
|---------|-----|
| **API URL** | `https://d3qfj76e9d3p81.cloudfront.net/api` |
| **WebSocket URL** | `wss://d3qfj76e9d3p81.cloudfront.net/ws` |
| **ECR Repository** | `202606334122.dkr.ecr.ap-northeast-1.amazonaws.com/hackz-ptera-back` |
| **ECS Cluster** | `hackz-ptera-cluster` |
| **ECS Service** | `hackz-ptera-service` |

### GitHub Secrets (設定済み)

- `AWS_ACCESS_KEY_ID`
- `AWS_SECRET_ACCESS_KEY`

---

## プロジェクト構成

```
back/
├── cmd/
│   └── server/          # エントリーポイント
├── internal/
│   ├── handler/         # HTTPハンドラー
│   ├── service/         # ビジネスロジック
│   ├── model/           # データモデル
│   └── websocket/       # WebSocket管理
├── Dockerfile           # 本番用
├── Dockerfile.dev       # 開発用 (ホットリロード)
├── .air.toml            # Air設定
├── go.mod
└── go.sum
```

---

## API エンドポイント

| メソッド | パス | 説明 |
|---------|------|------|
| GET | `/health` | ヘルスチェック |
| GET | `/ws` | WebSocket接続 |
| POST | `/api/queue/join` | 待機列に参加 |
| POST | `/api/captcha/verify` | CAPTCHA検証 |
| POST | `/api/otp/verify` | OTP検証 |
| POST | `/api/register` | 登録完了 |

---

## 開発ワークフロー

1. `develop` ブランチから feature ブランチを作成
2. 実装・テスト
3. `develop` へPR作成・マージ
4. 動作確認後 `main` へマージ → 本番自動デプロイ
