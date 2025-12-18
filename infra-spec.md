# インフラ設計書：The Frustrating Registration Form

## 1. 概要

| 項目 | 内容 |
|------|------|
| **利用期間** | 4日間（ハッカソン用） |
| **予算** | $100（AWSクレジット） |
| **想定コスト** | $7〜15 |
| **リージョン** | ap-northeast-1（東京） |

## 2. アーキテクチャ

```
┌─────────────────────────────────────────────────────────────────────────┐
│                              AWS Cloud                                  │
│                                                                         │
│  ┌───────────────────────────────────────────────────────────────────┐ │
│  │                      CloudFront (CDN)                              │ │
│  │                            │                                       │ │
│  │           ┌────────────────┼────────────────┐                      │ │
│  │           ▼                ▼                ▼                      │ │
│  │    [S3: Frontend]    [S3: Assets]      [ALB:80]                    │ │
│  │    (静的サイト)       (画像)          (API/WebSocket)              │ │
│  └───────────────────────────────────────────────────────────────────┘ │
│                                                                         │
│  ┌───────────────────────────────────────────────────────────────────┐ │
│  │                            VPC                                     │ │
│  │                     (10.0.0.0/16)                                  │ │
│  │                                                                    │ │
│  │   ┌─────────────────┐       ┌─────────────────┐                   │ │
│  │   │  Public Subnet  │       │  Public Subnet  │                   │ │
│  │   │   (10.0.1.0/24) │       │   (10.0.2.0/24) │                   │ │
│  │   │      AZ-a       │       │      AZ-c       │                   │ │
│  │   └────────┬────────┘       └────────┬────────┘                   │ │
│  │            │                         │                            │ │
│  │            └───────────┬─────────────┘                            │ │
│  │                        │                                          │ │
│  │                        ▼                                          │ │
│  │                 [ECS Fargate]                                     │ │
│  │                 (Single Task)                                     │ │
│  │                       │                                           │ │
│  │            ┌──────────┼──────────┐                                │ │
│  │            ▼          ▼          ▼                                │ │
│  │       [Bedrock]   [S3 Assets] [In-Memory]                         │ │
│  │     Claude 3 Haiku  (読取)    (状態管理)                           │ │
│  └───────────────────────────────────────────────────────────────────┘ │
│                                                                         │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐                     │
│  │     ECR     │  │ CloudWatch  │  │  IAM Role   │                     │
│  │   (Image)   │  │   (Logs)    │  │ (Bedrock/S3)│                     │
│  └─────────────┘  └─────────────┘  └─────────────┘                     │
└─────────────────────────────────────────────────────────────────────────┘
```

## 3. CI/CD パイプライン

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         GitHub Actions                                  │
│                                                                         │
│   [Push to main]                                                        │
│         │                                                               │
│         ▼                                                               │
│   ┌───────────┐    ┌───────────┐    ┌───────────┐    ┌───────────┐    │
│   │  Checkout │ →  │ Go Build  │ →  │  Docker   │ →  │ Push ECR  │    │
│   │           │    │  & Test   │    │   Build   │    │           │    │
│   └───────────┘    └───────────┘    └───────────┘    └───────────┘    │
│                                                               │         │
│                                                               ▼         │
│                                                        ┌───────────┐   │
│                                                        │  Deploy   │   │
│                                                        │  to ECS   │   │
│                                                        └───────────┘   │
└─────────────────────────────────────────────────────────────────────────┘
```

## 4. AWSリソース詳細

### 4.1 ネットワーク

| リソース | 設定 | 備考 |
|----------|------|------|
| **VPC** | 10.0.0.0/16 | 1つ作成 |
| **Public Subnet** | 10.0.1.0/24 (AZ-a) | ALB/ECS用 |
| **Public Subnet** | 10.0.2.0/24 (AZ-c) | ALB用（Multi-AZ必須） |
| **Internet Gateway** | - | VPCにアタッチ |
| **Route Table** | 0.0.0.0/0 → IGW | Public Subnet用 |
| **NAT Gateway** | ❌ 使用しない | コスト削減のため |

### 4.2 セキュリティグループ

**ALB用:**
| 方向 | ポート | ソース | 用途 |
|------|--------|--------|------|
| Inbound | 80 | 0.0.0.0/0 | HTTP |
| Inbound | 443 | 0.0.0.0/0 | HTTPS（将来用） |
| Outbound | All | 0.0.0.0/0 | - |

**ECS用:**
| 方向 | ポート | ソース | 用途 |
|------|--------|--------|------|
| Inbound | 8080 | ALB SG | アプリケーション |
| Outbound | All | 0.0.0.0/0 | Bedrock/S3アクセス |

### 4.3 ALB (Application Load Balancer)

| 項目 | 値 |
|------|-----|
| **スキーム** | internet-facing |
| **リスナー** | HTTP:80 → ターゲットグループ |
| **ターゲットグループ** | IP type, Port 8080 |
| **ヘルスチェック** | GET /health |
| **Idle Timeout** | 300秒（WebSocket用） |
| **Stickiness** | 無効（Single Task） |

### 4.4 ECS

| 項目 | 値 |
|------|-----|
| **クラスター** | Fargate |
| **タスク定義** | 0.5 vCPU / 1GB RAM |
| **サービス** | Desired Count: 1 |
| **Auto Scaling** | 無効 |
| **ネットワークモード** | awsvpc |
| **パブリックIP** | 有効（NAT Gateway不使用のため） |

### 4.5 ECR

| 項目 | 値 |
|------|-----|
| **リポジトリ名** | hackz-ptera-back |
| **イメージタグ** | latest, git-sha |
| **ライフサイクル** | 最新5イメージのみ保持 |

### 4.6 S3

**アセットバケット（バックエンド用）:**
| 項目 | 値 |
|------|-----|
| **バケット名** | hackz-ptera-assets-{account-id} |
| **用途** | 魚画像、キャラクター画像、背景画像 |
| **アクセス** | ECSタスクロール + CloudFront OAC |

**フロントエンドバケット:**
| 項目 | 値 |
|------|-----|
| **バケット名** | hackz-ptera-frontend-{account-id} |
| **用途** | 静的ウェブサイト（HTML/CSS/JS） |
| **アクセス** | CloudFront OACからのみ |

### 4.7 CloudFront

| 項目 | 値 |
|------|-----|
| **ディストリビューション** | 1つ作成 |
| **オリジン1** | S3 Frontend（デフォルト） |
| **オリジン2** | S3 Assets（/assets/*） |
| **オリジン3** | ALB（/api/*, /ws） |
| **キャッシュ** | 静的ファイルのみ（API/WSはキャッシュ無効） |
| **OAC** | S3アクセス用に設定 |

**ビヘイビア設定:**
| パスパターン | オリジン | キャッシュ | WebSocket |
|--------------|----------|------------|-----------|
| `/api/*` | ALB | 無効 | - |
| `/ws` | ALB | 無効 | ✅ 有効 |
| `/assets/*` | S3 Assets | 有効 | - |
| `/*` (デフォルト) | S3 Frontend | 有効 | - |

### 4.8 IAM

**ECSタスク実行ロール:**
- `AmazonECSTaskExecutionRolePolicy`
- ECRからのイメージプル
- CloudWatch Logsへの書き込み

**ECSタスクロール:**
- `bedrock:InvokeModel`（Claude 3 Haiku のみ）
- `s3:GetObject`（アセットバケット）

## 5. コスト詳細

### 5.1 インフラ基盤（4日間 = 96時間）

| リソース | 単価 | 計算 | コスト |
|----------|------|------|--------|
| ECS Fargate (vCPU) | $0.02571/時 | × 0.5 × 96h | $1.23 |
| ECS Fargate (RAM) | $0.00283/時 | × 1GB × 96h | $0.27 |
| ALB | $0.0243/時 | × 96h | $2.33 |
| ALB (LCU) | $0.008/LCU-時 | × 0.5 × 96h | $0.38 |
| **小計** | | | **$4.21** |

### 5.2 ストレージ・転送・CDN

| リソース | 単価 | 計算 | コスト |
|----------|------|------|--------|
| ECR ストレージ | $0.10/GB/月 | 100MB × 4日 | $0.01 |
| S3 ストレージ (Assets) | $0.025/GB/月 | 50MB × 4日 | $0.01 |
| S3 ストレージ (Frontend) | $0.025/GB/月 | 10MB × 4日 | $0.01 |
| CloudWatch Logs | $0.76/GB | 500MB | $0.38 |
| CloudFront | - | 1TB無料枠内 | $0.00 |
| データ転送 | - | 最初の100GB無料 | $0.00 |
| **小計** | | | **$0.41** |

### 5.3 Bedrock（想定使用量）

| 項目 | 単価 | 想定量 | コスト |
|------|------|--------|--------|
| Claude 3 Haiku 入力 | $0.25/100万トークン | 500K | $0.13 |
| Claude 3 Haiku 出力 | $1.25/100万トークン | 200K | $0.25 |
| **小計** | | | **$0.38** |

### 5.4 CI/CD

| サービス | コスト | 備考 |
|----------|--------|------|
| GitHub Actions | $0 | 2000分/月無料 |

### 5.5 合計

| カテゴリ | コスト |
|----------|--------|
| インフラ基盤 | $4.21 |
| ストレージ・転送・CDN | $0.41 |
| Bedrock | $0.38 |
| CI/CD | $0.00 |
| **合計** | **$5.00** |
| **バッファ込み（2倍）** | **$10.00** |

### 5.6 予算サマリ

| 項目 | 金額 |
|------|------|
| 予算 | $100 |
| 想定コスト | $5〜10 |
| 安全マージン | $90以上 |
| **判定** | ✅ **十分足りる** |

## 6. GitHub Actions ワークフロー

```yaml
name: Deploy to ECS

on:
  push:
    branches: [main]

env:
  AWS_REGION: ap-northeast-1
  ECR_REPOSITORY: hackz-ptera-back
  ECS_CLUSTER: hackz-ptera-cluster
  ECS_SERVICE: hackz-ptera-service

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.24'

      - name: Build and Test
        run: |
          go build ./...
          go test ./...

      - name: Configure AWS credentials
        uses: aws-actions/configure-aws-credentials@v4
        with:
          aws-access-key-id: ${{ secrets.AWS_ACCESS_KEY_ID }}
          aws-secret-access-key: ${{ secrets.AWS_SECRET_ACCESS_KEY }}
          aws-region: ${{ env.AWS_REGION }}

      - name: Login to Amazon ECR
        id: login-ecr
        uses: aws-actions/amazon-ecr-login@v2

      - name: Build and push Docker image
        env:
          ECR_REGISTRY: ${{ steps.login-ecr.outputs.registry }}
          IMAGE_TAG: ${{ github.sha }}
        run: |
          docker build -t $ECR_REGISTRY/$ECR_REPOSITORY:$IMAGE_TAG .
          docker build -t $ECR_REGISTRY/$ECR_REPOSITORY:latest .
          docker push $ECR_REGISTRY/$ECR_REPOSITORY:$IMAGE_TAG
          docker push $ECR_REGISTRY/$ECR_REPOSITORY:latest

      - name: Deploy to ECS
        run: |
          aws ecs update-service \
            --cluster $ECS_CLUSTER \
            --service $ECS_SERVICE \
            --force-new-deployment
```

## 7. 構築手順

### Phase 1: 事前準備
1. [x] AWS CLIのセットアップ
2. [x] GitHub Secretsの設定（AWS認証情報）

### Phase 2: インフラ構築
1. [x] VPC/Subnet/Security Group作成
2. [x] ALB作成
3. [x] ECRリポジトリ作成
4. [x] ECSクラスター/サービス作成
5. [x] S3バケット作成（Assets + Frontend）
6. [x] CloudFront作成（S3 + ALBオリジン）
7. [x] IAMロール作成

### Phase 3: CI/CD設定
1. [x] Dockerfile作成
2. [x] GitHub Actionsワークフロー作成
3. [x] 初回デプロイテスト

### Phase 4: 後片付け（ハッカソン終了後）
1. [ ] CloudFront削除
2. [ ] ECSサービス削除
3. [ ] ALB削除
4. [ ] ECRイメージ削除
5. [ ] S3バケット削除（Assets + Frontend）
6. [ ] VPC削除

## 8. 注意事項

### コスト管理
- ⚠️ **使用後は必ずリソースを削除**
- ALB/ECSを放置すると課金継続
- CloudWatchで請求アラートを設定推奨

### セキュリティ
- GitHub Secretsに認証情報を保存
- IAMは最小権限の原則
- S3は非公開設定

### 制限事項
- NAT Gatewayは使用しない（コスト削減）
- Auto Scalingは無効（Single Task運用）
- HTTPSは今回は省略（本番では必須）
