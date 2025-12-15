# バックエンド開発仕様書：The Frustrating Registration Form (AWS Edition) v6

## 1. プロジェクト概要
ユーザーの忍耐力を極限まで試すジョークWebアプリケーション。
ディズニーランド級の「待機列」の後、2段階のミニゲーム（アクション＆視覚テスト）をクリアしないと会員登録フォームに辿り着けない。
どの段階で失敗しても、ペナルティとして待機列の最後尾からやり直しとなる。

**鬼畜仕様:** すべての関門をクリアしても、最後に「サーバーエラー」で登録は永遠に完了しない。

## 2. 技術スタック
* **言語:** Go (Latest) / Echo (v4)
* **AWS:** ECS Fargate + ALB, Bedrock (Claude 3 Haiku), S3, CloudFront
* **画像処理:** Go標準ライブラリ (image/draw)
* **通信:** gorilla/websocket

## 3. インフラ方針
* **構成:** Single Task (Auto Scaling Off)
* **データ:** In-Memory (再起動で全データロスト)
* **利用期間:** 4日間（ハッカソン用）
* **注意:** App RunnerはWebSocket非対応のため、ECS Fargate + ALBを採用

## 4. データ構造 (In-Memory)

### User構造体
```go
type User struct {
    ID            string          // UUID
    SessionID     string          // セッションID（Cookie）
    Conn          *websocket.Conn
    JoinedAt      time.Time
    Status        string          // 現在の状況

    // CAPTCHA用
    CaptchaTargetX  int
    CaptchaTargetY  int
    CaptchaAttempts int  // 試行回数（最大3回）

    // OTP用
    OTPFishName   string    // 正解の魚名
    OTPAttempts   int       // 試行回数（最大3回）

    // 登録用
    RegisterToken    string
    RegisterTokenExp time.Time  // 有効期限（10分）
}

// Status Enum:
// "waiting"         : 待機列で待機中
// "stage1_dino"     : 第1関門 Dino Run プレイ中
// "stage2_captcha"  : 第2関門 CAPTCHA プレイ中
// "registering"     : 登録フォーム入力中 (クリア済)
```

### セッション管理
```go
type SessionStore struct {
    sessions map[string]*User  // sessionID -> User
    mu       sync.RWMutex
}
```

### 待機列管理
```go
type WaitingQueue struct {
    users []*User  // 待機中のユーザー（順番通り）
    mu    sync.RWMutex
}
```

## 5. APIエンドポイント & フロー詳細

### Phase 1: 終わらない待機列 (WebSocket)

**Endpoint:** `GET /ws`

* **接続:** ユーザーを `waiting` 状態で WaitingQueue の最後尾に追加。
* **進行ロジック:** 1人ずつ順番に進む（前の人がゲームを終えるまで次の人は待機）
* **通知タイミング:** 順番が変わったときのみ通知
* **関門突破:**
  * 人数が0になり、かつランダムな焦らし時間（10〜30秒）が経過したら、第1関門へ誘導。

**焦らし時間の詳細フロー:**
1. 待機列が0人になる
2. 10〜30秒のランダム待機（焦らし時間）開始
3. この間に新規ユーザーが来ても、タイマーは継続（**先着優先**）
4. 新規ユーザーは待機列1人目として待機
5. タイマー完了後、先頭ユーザーがステージ1へ

```json
// Server -> Client
{ "type": "queue_update", "position": 5, "message": "現在5人待ちです..." }
{ "type": "stage_change", "status": "stage1_dino", "message": "接続安定性を確認するためのテストを行います..." }
```

### Phase 2: 第1関門 - Dino Run (アクション)

**Endpoint:** `POST /api/game/dino/result`

* **概要:** Chrome恐竜ゲームの激ムズ版。
* **不正対策:** なし（ジョークアプリのため）
* **タイムアウト:** 3分（結果未送信の場合、失敗扱い）
* **再試行:** 不可（1回のみ）

**Request:**
```json
{ "score": 500, "survived": true }
```

**Logic (The Filter):**

* **タイムアウト（3分経過）:**
  * サーバーからWebSocket経由で失敗通知を送信。
  * WebSocket切断。
  * **結果:** 待機列の最後尾へリセット。
* **失敗 (`survived: false`):**
  * 失敗メッセージを送信後、WebSocket切断。
  * クライアントは3秒後に自動でトップページへリダイレクト。
  * **結果:** 待機列の最後尾へリセット。
* **成功 (`survived: true`):**
  * ユーザーのステータスを `stage2_captcha` に更新。

**Response (成功時):**
```json
{ "error": false, "next_stage": "captcha", "message": "身体能力テスト合格。次は視力テストです。" }
```

**Response (失敗時):**
```json
{ "error": true, "message": "ゲームオーバー。待機列の最後尾からやり直しです。", "redirect_delay": 3 }
```

### Phase 3: 第2関門 - Impossible CAPTCHA

アクションゲームで疲れた目に追い打ちをかける。

* **タイムアウト:** 3分（時間切れで失敗扱い）
* **再試行:** 3回まで可能

**Endpoint:** `GET /api/captcha/generate`

* **前提:** Userのステータスが `stage2_captcha` であること。
* **Logic:**
  * S3から背景画像をランダム取得 + 極小オリジナルキャラクターを合成。
  * 正解座標をUser構造体に保存。

**Response:**
```json
{
  "error": false,
  "image_url": "https://xxx.cloudfront.net/captcha/xxxxx.png",
  "message": "画像の中に隠れているキャラクターをクリックしてください"
}
```

**Endpoint:** `POST /api/captcha/verify`

**Request:**
```json
{ "x": 123, "y": 456 }
```

**Logic (The Trap):**

* 許容範囲（半径5px〜10px）判定。
* **タイムアウト（3分経過）:**
  * サーバーからWebSocket経由で失敗通知を送信。
  * WebSocket切断。
  * **結果:** 待機列の最後尾へリセット。
* **失敗（1〜2回目）:**
  * 残り試行回数を通知。
  * 新しいCAPTCHA画像を生成。
* **失敗（3回目）:**
  * 失敗メッセージを送信後、WebSocket切断。
  * クライアントは3秒後に自動でトップページへリダイレクト。
  * **ユーザー体験:** 「あと少しで登録できたのに、また150人待ちの最初から！？」
* **成功:**
  * ステータスを `registering` に更新。
  * 登録用トークン（UUID）を発行、有効期限10分。

**Response (成功時):**
```json
{ "error": false, "token": "550e8400-e29b-41d4-a716-446655440000", "message": "視力テスト合格！登録フォームへどうぞ。" }
```

**Response (失敗時・1〜2回目):**
```json
{
  "error": true,
  "message": "不正解です。残り2回",
  "attempts_remaining": 2,
  "new_image_url": "https://xxx.cloudfront.net/captcha/newimage.png"
}
```

**Response (失敗時・3回目):**
```json
{
  "error": true,
  "message": "3回失敗しました。待機列の最後尾からやり直しです。",
  "redirect_delay": 3
}
```

### Phase 4: 会員登録 (The Final Boss)

**Endpoint:** `POST /api/register`

* トークン所有者のみアクセス可能。
* トークンはSessionIDと紐付けて検証。
* **トークン有効期限:** 10分（期限切れで待機列の最後尾へ）

**Request:**
```json
{
  "token": "550e8400-e29b-41d4-a716-446655440000",
  "name": "田中太郎",
  "email": "taro@example.com",
  "password": "password123",
  "birthday": "1998-03-15",
  "phone": "090-1234-5678",
  "address": "東京都渋谷区..."
}
```
※ 詳細な入力項目は後で指定

#### AIパスワード煽り (`POST /api/password/analyze`)

* Bedrock (Claude 3 Haiku) がパスワードをリアルタイムで分析。
* **タイミング:** デバウンス（500ms遅延）でAPI呼び出し
* **アクセス制限:** なし（SessionIDがあれば誰でも利用可能）
* 入力された文字列から名前や生年月日などを予測し、イライラするメッセージを生成。
* 例: 「ここまで必死になって辿り着いたのに、そのパスワードｗｗ」「もしかして誕生日1998年3月15日？」

**Request:**
```json
{ "password": "taro1998" }
```

**Response:**
```json
{ "error": false, "message": "太郎さんですか？1998年生まれ？そのパスワード、3秒で破られますよ..." }
```

**Bedrockエラー時のフォールバック:**
```go
var fallbackMessages = []string{
    "そのパスワード、弱そうですね...",
    "もう少し工夫してみては？",
    "予測しやすそうなパスワードですね。",
}
```
※ Bedrockがエラーの場合、上記からランダムに1つ返す

#### 魚OTP (`POST /api/otp/send` -> `POST /api/otp/verify`)

* **S3に事前保存した魚画像セット**（約20種のマイナー魚）をランダム表示。
* ユーザーは魚の品種名を当てて入力する。
* **正解判定:** ひらがな/カタカナ許容（「おにかます」「オニカマス」どちらもOK）
* **失敗時:** 3回まで再試行可能。**毎回新しい魚**を表示。3回失敗で待機列の最後尾へ。

**POST /api/otp/send Response:**
```json
{ "error": false, "image_url": "https://xxx.cloudfront.net/fish/onikamasu.jpg", "message": "この魚の名前を入力してください" }
```

**POST /api/otp/verify Request:**
```json
{ "answer": "オニカマス" }
```

**POST /api/otp/verify Response (成功時):**
```json
{ "error": false, "message": "正解！登録を完了してください。" }
```

**POST /api/otp/verify Response (失敗時・1〜2回目):**
```json
{
  "error": true,
  "message": "不正解です。残り2回",
  "attempts_remaining": 2,
  "new_image_url": "https://xxx.cloudfront.net/fish/newfish.jpg"
}
```

**POST /api/otp/verify Response (失敗時・3回目):**
```json
{
  "error": true,
  "message": "3回失敗しました。待機列の最後尾からやり直しです。",
  "redirect_delay": 3
}
```

#### 登録完了（鬼畜仕様）

**POST /api/register** を呼んだ後、すべてのバリデーションが成功しても：

```json
{
  "error": true,
  "message": "サーバーエラーが発生しました。お手数ですが最初からやり直してください。",
  "redirect_delay": 3
}
```

* WebSocket切断
* 3秒後にトップページへリダイレクト
* **永遠に登録は完了しない**

## 6. クライアント(Front)への実装要求

### 戻るボタン無効化
* ゲーム中に「戻る」を押したら、APIを叩かずに即座にトップページ（待機列）へ飛ばす。

### 恐怖の演出
* 第2関門（CAPTCHA）の説明文に、赤文字で**「※失敗すると待機列の最後尾に戻ります」**と小さく表示し、プレッシャーを与える。

### 失敗時の自動リダイレクト
* `redirect_delay` が含まれるレスポンスを受け取ったら、指定秒数後にトップページへリダイレクト。

---

## 7. 認証・セッション管理

| 項目 | 内容 |
|------|------|
| **方式** | Cookie（セッションID） |
| **発行タイミング** | WebSocket接続時 |
| **有効期限** | セッション（ブラウザを閉じるまで） |
| **用途** | WebSocket接続とREST APIの紐付け |

## 8. 登録トークン仕様

| 項目 | 値 |
|------|-----|
| **形式** | UUID v4 |
| **有効期限** | 10分 |
| **保存場所** | User構造体内 |
| **検証** | SessionID + Token の一致確認 |

## 9. CAPTCHA詳細仕様

| 項目 | 値 |
|------|-----|
| **画像サイズ** | 1024 x 768 px |
| **ターゲットサイズ** | 5〜8 px |
| **ターゲット** | オリジナルキャラクター（S3から取得） |
| **許容範囲** | 半径 5〜10 px |
| **背景** | S3に事前保存した画像（約20種）をランダム使用 |

## 10. 魚OTP詳細仕様

| 項目 | 内容 |
|------|------|
| **画像ソース** | S3に事前保存した魚画像セット（約20種） |
| **魚の例** | オニカマス、ホウボウ、マツカサウオ、ハリセンボン等 |
| **正解判定** | ひらがな/カタカナ許容、大文字小文字無視 |
| **再試行** | 3回まで可能 |
| **失敗ペナルティ** | 3回失敗で待機列の最後尾へ |

## 11. WebSocket仕様

### メッセージ形式

**Server → Client:**
```json
{ "type": "queue_update", "position": 5, "total": 150 }
{ "type": "stage_change", "status": "stage1_dino", "message": "..." }
{ "type": "error", "code": "SESSION_EXPIRED", "message": "..." }
{ "type": "failure", "message": "...", "redirect_delay": 3 }
```

**Client → Server:**
```json
{ "type": "ping" }
```

### 接続維持
| 項目 | 値 |
|------|-----|
| **Ping間隔** | 30秒（クライアントから送信） |
| **ALB Idle Timeout** | 300秒 |
| **通知タイミング** | 順番が変わったときのみ |

### 再接続ポリシー

| 切断理由 | 再接続 | 結果 |
|----------|--------|------|
| **ネットワーク障害** | ❌ 不可 | 最後尾から |
| **ブラウザリロード** | ❌ 不可 | 最後尾から |
| **ステージ失敗** | ❌ 不可 | 最後尾から |
| **タイムアウト** | ❌ 不可 | 最後尾から |

※ いかなる理由でも切断された場合、再接続時は待機列の最後尾に並び直し

## 12. タイムアウト・再試行仕様

### タイムアウト

| ステージ | タイムアウト | 結果 |
|----------|--------------|------|
| **Dino Run** | 3分 | 失敗扱い、最後尾へ |
| **CAPTCHA** | 3分 | 失敗扱い、最後尾へ |
| **登録フォーム** | 10分（トークン有効期限） | 失敗扱い、最後尾へ |

### 再試行

| ステージ | 再試行回数 | 失敗時の挙動 |
|----------|------------|--------------|
| **Dino Run** | 1回のみ | 即失敗、最後尾へ |
| **CAPTCHA** | 3回まで | 同じ画像で再試行、3回失敗で最後尾へ |
| **魚OTP** | 3回まで | **毎回新しい魚**、3回失敗で最後尾へ |

## 13. エラーレスポンス形式

すべてのREST APIで統一した形式を使用。

**成功時:**
```json
{ "error": false, "data": {...} }
```

**失敗時:**
```json
{ "error": true, "message": "エラーメッセージ" }
```

**リダイレクト付き失敗:**
```json
{ "error": true, "message": "...", "redirect_delay": 3 }
```

## 14. CORS設定

| 項目 | 値 |
|------|-----|
| **Allowed Origins** | CloudFrontドメイン |
| **Allowed Methods** | GET, POST, OPTIONS |
| **Allowed Headers** | Content-Type, Cookie |
| **Credentials** | true（Cookie送信のため） |

---

## 15. インフラ構成詳細

```
┌─────────────────────────────────────────────────────────────────────┐
│                           AWS Cloud                                 │
│                                                                     │
│  [CloudFront] ──→ [S3: Frontend]                                   │
│       │                                                             │
│       └──→ [ALB] ──→ [ECS Fargate]                                 │
│             │         (Single Task)                                 │
│             │              │                                        │
│       ┌─────┴─────┐        │                                        │
│       │           │        │                                        │
│  HTTP/WS     Health      ┌─┴──────────────┐                         │
│  :8080       Check       │                │                         │
│                          ↓                ↓                         │
│                    [Bedrock]          [S3: Assets]                  │
│                 Claude 3 Haiku        - 魚画像                      │
│                 (ap-northeast-1)      - キャラ画像                  │
│                                       - 背景画像                    │
│                                                                     │
│  [ECR] ← Docker Image                                              │
│  [CloudWatch] ← Logs                                               │
└─────────────────────────────────────────────────────────────────────┘
```

### 必要なAWSリソース

| リソース | 用途 | 設定 |
|----------|------|------|
| **VPC** | ネットワーク | Public Subnet x2 (Multi-AZ) |
| **ALB** | ロードバランサー | WebSocket対応、Idle Timeout: 300秒 |
| **ECS Cluster** | コンテナ管理 | Fargate |
| **ECS Service** | タスク実行 | Desired Count: 1, Auto Scaling: Off |
| **ECR** | Dockerイメージ | プライベートリポジトリ |
| **S3 (Assets)** | 静的ファイル | 魚画像、キャラクター画像、背景画像 |
| **S3 (Frontend)** | フロントエンド | 静的ウェブサイト |
| **CloudFront** | CDN | S3 + ALBをオリジン |
| **IAM Role** | 権限 | Bedrock呼び出し、S3読み取り |
| **CloudWatch** | ログ | ECSタスクログ |

### ALB設定

| 項目 | 値 |
|------|-----|
| **Idle Timeout** | 300秒（待機列で長時間接続を維持） |
| **Stickiness** | 不要（Single Taskのため） |
| **Health Check** | `GET /health` |

### コスト概算（東京リージョン・4日間）

| リソース | 単価 | 4日間コスト |
|----------|------|-------------|
| ECS Fargate (0.5 vCPU, 1GB) | $0.03/時 | 約$3 |
| ALB | $0.03/時 + LCU | 約$3 |
| Bedrock Claude 3 Haiku | $0.25/100万入力トークン | 約$0.5 |
| S3 | - | ほぼ無料 |
| CloudFront | - | ほぼ無料（1TB無料枠） |
| **合計** | | **約$7〜15** |

※ 使用後はリソースを削除してコストを抑える
