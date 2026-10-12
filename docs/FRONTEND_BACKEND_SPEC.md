# Messanger — Backend API Specification for Frontend

Документ описывает **публичный контракт бэкенда** для реализации веб/мобильного клиента. Источник истины по полям — protobuf в `shared/proto/**`; HTTP — Envoy gRPC-JSON transcoder; realtime — Gateway WebSocket.

**Прод:** `https://chat.arbuz-arbuz.site`  
**Локально (типично):** `http://localhost:8088` (web nginx → `/api` → Envoy, `/ws` → Gateway)

---

## 1. Архитектура с точки зрения фронта

```
Browser
  ├─ HTTPS REST  /api/v1/...  → Envoy (gRPC-JSON) → user | auth | chat | media | search | notify
  ├─ WSS         /ws          → Gateway (JWT) → presence / typing / chat.realtime fanout
  └─ Media PUT   putUrl (presigned MinIO/S3) → object storage
                 GET getUrl (presigned or path-style via same origin /messanger/...)
```

Web nginx (в контейнере `messanger-web`) проксирует:

| Path | Backend |
|------|---------|
| `/api/` | Envoy `:8080` |
| `/ws`, `/v1/ws` | Gateway `:8082` |
| `/messanger/` | MinIO (path-style public objects) |
| `/*` | SPA |

Фронту **не нужно** ходить в gRPC напрямую. Только HTTP JSON + WebSocket JSON.

---

## 2. Соглашения JSON / HTTP

### 2.1 Именование полей

Envoy: `preserve_proto_field_names: false` → **camelCase** в JSON:

| Proto | JSON |
|-------|------|
| `access_token` | `accessToken` |
| `chat_id` | `chatId` |
| `avatar_file_id` | `avatarFileId` |
| `is_pinned` | `isPinned` |

**Исключение: WebSocket realtime** шлёт **snake_case** (`chat_id`, `message_id`, `attachment_ids`, …). Клиент должен принимать оба стиля.

### 2.2 Числа `int64`

gRPC-JSON часто кодирует `int64` как **строку** (`"42"`). Всегда нормализуйте:

```ts
const n = typeof v === 'number' ? v : Number(v)
```

### 2.3 Timestamps

RFC3339 / protobuf Timestamp → строка ISO-8601, например `"2026-10-02T20:13:24.118Z"`.

### 2.4 Enums

Могут прийти как **имя** (`"CHAT_TYPE_DIRECT"`) или **число** (`1`). Проверяйте оба.

### 2.5 Пустые ответы

Методы `google.protobuf.Empty` → HTTP `200` с пустым/`{}` телом (иногда без тела). Не полагайтесь на `204`.

### 2.6 Ошибки

- HTTP статус из gRPC (`Unauthenticated` → 401, `NotFound` → 404, `InvalidArgument` → 400, `PermissionDenied` → 403, …).
- Тело часто: `{ "code": number|string, "message": "...", "details": [...] }` (grpc-json `convert_grpc_status`).
- Читайте `message` / `error`.

### 2.7 Content-Type

`Content-Type: application/json` для всех JSON body.

---

## 3. Аутентификация

### 3.1 Токены

| Токен | Назначение | Где хранить (рекомендация) |
|-------|------------|----------------------------|
| `accessToken` | Bearer для REST + WS | memory / sessionStorage + cookie для WS |
| `refreshToken` | Обновление access / logout | persistent storage |

**JWT access claims (важное):**

```json
{
  "user_id": "123",
  "exp": 1710000000,
  "iat": 1709999100
}
```

- Claim пользователя: **`user_id`** (строка с числовым id).
- Алгоритм: HS256 (серверная сторона).
- Типичный TTL access: ~15m; refresh: дни (настраивается сервером).

### 3.2 Заголовок

```
Authorization: Bearer <accessToken>
```

Требуется почти везде, кроме:

- `POST /api/v1/auth/login`
- `POST /api/v1/auth/refresh`
- `POST /api/v1/auth/access`
- `POST /api/v1/users` (регистрация)

`GET /api/v1/auth/validate` — с Bearer (проверка access).

### 3.3 Рекомендуемый refresh-flow

1. При `401` на защищённом запросе → `POST /api/v1/auth/access` с `{ "refreshToken" }`.
2. Получить новый `accessToken`, повторить исходный запрос.
3. Если refresh тоже 401 → разлогинить.

### 3.4 WebSocket auth

Gateway принимает токен так (по приоритету):

1. `Authorization: Bearer …` (браузер на WS поставить не может),
2. query `?token=<accessToken>` (используется текущим web-клиентом),
3. опционально cookie `access_token` — web nginx может проставить `Authorization` из cookie при прокси `/ws`.

Практика текущего SPA:

```ts
document.cookie = `access_token=${encodeURIComponent(token)}; path=/; SameSite=Lax`
const url = `${wss}://${host}/ws?token=${encodeURIComponent(token)}`
```

---

## 4. Auth API

### `POST /api/v1/auth/login` (public)

```json
{ "email": "a@b.c", "password": "secret12" }
```

```json
{ "accessToken": "...", "refreshToken": "..." }
```

Ошибки: неверные credentials → Unauthenticated.

### `POST /api/v1/auth/access` (public)

```json
{ "refreshToken": "..." }
```

```json
{ "accessToken": "..." }
```

### `POST /api/v1/auth/refresh` (public)

Ротация refresh:

```json
{ "refreshToken": "..." }
```

```json
{ "refreshToken": "..." }
```

### `POST /api/v1/auth/logout`

```json
{ "refreshToken": "..." }
```

Инвалидирует refresh. Access доживёт до `exp`.

### `GET /api/v1/auth/validate`

Bearer access. Пустой ответ при успехе.

---

## 5. Users API

### Модель `User`

```ts
type User = {
  id: number | string
  userInfo?: {
    name?: string
    email?: string
    username?: string
  }
  createdAt?: string
  updatedAt?: string
  avatarFileId?: string  // media file_id, не URL
}
```

### Правила username

- Нормализация: trim, снять ведущий `@`, **lowercase**.
- Regex: `^[a-zA-Z][a-zA-Z0-9_]{2,31}$` → длина 3–32, начинается с буквы.
- Запрещены: `admin`, `support`, `system`, `me`, `api`, `root`, `null`, `undefined`.
- Уникальность вместе с email.

### Пароль

- Минимум **8** символов.
- При регистрации/смене: `password` === `passwordConfirm`.

### `POST /api/v1/users` — регистрация (public)

```json
{
  "userInfo": { "name": "Ada", "email": "ada@ex.com", "username": "ada" },
  "password": "secret12",
  "passwordConfirm": "secret12"
}
```

```json
{ "id": "42" }
```

После регистрации клиент обычно сразу делает `login`.

### `GET /api/v1/users/me`

```json
{ "user": { /* User */ } }
```

### `GET /api/v1/users/{id}`

### `GET /api/v1/users/by-username/{username}`

`username` в path без `@` (можно URL-encode).

### `GET /api/v1/users:search?query=&limit=`

- `query` — подстрока по username/name (ILIKE).
- `limit` — опционально (клиент обычно 20).
- Точное совпадение username поднимается выше в выдаче.

```json
{ "users": [ /* User */ ] }
```

### `PATCH /api/v1/users/me`

Частичное обновление. Для protobuf `StringValue` в JSON достаточно передать строку поля:

```json
{ "name": "Ada Lovelace", "username": "ada", "avatarFileId": "<file_id>" }
```

Передавайте только изменяемые поля.

### `POST /api/v1/users/me/password`

```json
{ "password": "newsecret1", "passwordConfirm": "newsecret1" }
```

### `DELETE /api/v1/users/me`

Удаление аккаунта текущего пользователя.

---

## 6. Chats & Messages API

### Enums

**ChatType**

| Значение | Смысл |
|----------|--------|
| `CHAT_TYPE_UNSPECIFIED` / `0` | не использовать |
| `CHAT_TYPE_DIRECT` / `1` | личка 1:1 |
| `CHAT_TYPE_GROUP` / `2` | группа |

**Role** (участник чата)

| Значение | Смысл |
|----------|--------|
| `ROLE_USER` / `0` | участник |
| `ROLE_ADMIN` / `1` | админ |
| `ROLE_OWNER` / `2` | владелец |

Права «админских» действий: `role >= 1` (ADMIN или OWNER).

### Модель `Chat` (в списках)

```ts
type Chat = {
  chatId: number
  ownerId: number
  chatInfo?: { name?: string; description?: string; userIds?: number[] }
  lastMessageAt?: string
  lastMessageId?: number
  unreadCount?: number
  chatType?: ChatType
  lastMessagePreview?: string
  participantIds?: number[]
  peerUserId?: number      // только DIRECT: id собеседника
  avatarFileId?: string    // аватар группы (DIRECT — обычно пусто, берите аватар peer)
  isPinned?: boolean       // pin чата для текущего viewer
}
```

### Модель `Message`

```ts
type Message = {
  messageId: number
  senderId: number
  chatId: number
  text: string
  isPinned?: boolean
  sendAt?: string
  updatedAt?: string
  attachmentIds?: string[]     // media file_id[]
  replyToMessageId?: number
}
```

### Бизнес-правила чатов

| Действие | Правила |
|----------|---------|
| Создать группу `POST /chats` | Нужны **имя** и ≥1 другой участник; создатель = OWNER |
| Direct `POST /chats:direct` | `peerUserId` ≠ self; идемпотентно возвращает существующий DM |
| Добавить/удалить участника | Только ADMIN/OWNER; **нельзя** в DIRECT |
| Удалить владельца | Нельзя |
| Назначить OWNER | Только текущий OWNER |
| Update chat (name/desc/avatar) | ADMIN/OWNER |
| Pin chat | Любой участник; pin **per-user** |
| Send/edit/delete/pin message | Участник; edit/delete — обычно автор (сервер проверяет) |
| `idempotencyKey` | Рекомендуется UUID на каждое «логическое» send; повтор с тем же ключом вернёт тот же `messageId` |

Сортировка списка чатов (клиентская рекомендация, как в текущем SPA):

1. `isPinned` сначала,
2. затем по `lastMessageAt` desc.

---

### `POST /api/v1/chats` — создать группу

```json
{
  "chatInfo": {
    "name": "Team",
    "description": "…",
    "userIds": [2, 3]
  }
}
```

```json
{ "chatId": "10" }
```

### `POST /api/v1/chats:direct`

```json
{ "peerUserId": 7 }
```

```json
{ "chatId": "11" }
```

### `GET /api/v1/chats?cursor=&limit=`

```json
{
  "chats": [ /* Chat */ ],
  "nextCursor": ""
}
```

Пагинация cursor-based. Пустой `nextCursor` = конец.

### `GET /api/v1/chats/{chatId}`

```json
{
  "chat": { /* Chat */ },
  "participantIds": [1, 2, 3],
  "myRole": "ROLE_OWNER"
}
```

### `PATCH /api/v1/chats/{chatId}`

```json
{
  "name": "New name",
  "description": "…",
  "avatarFileId": "<file_id>"
}
```

Только изменённые поля.

### `DELETE /api/v1/chats/{chatId}`

Soft-delete (права на стороне сервера).

### `GET /api/v1/chats:ids`

```json
{ "chatIds": ["1", "2"] }
```

Используется gateway для membership; фронту редко нужно.

### `POST /api/v1/chats/{chatId}/members`

```json
{ "chatId": 10, "userId": 5, "role": "ROLE_USER" }
```

### `DELETE /api/v1/chats/{chatId}/members/{userId}`

### `PATCH /api/v1/chats/{chatId}/members/{userId}`

```json
{ "chatId": 10, "userId": 5, "role": "ROLE_ADMIN" }
```

### `POST /api/v1/chats/{chatId}/messages`

```json
{
  "chatId": 10,
  "text": "hello",
  "idempotencyKey": "550e8400-e29b-41d4-a716-446655440000",
  "attachmentIds": ["file-uuid-1"],
  "replyToMessageId": 99
}
```

- `text` может быть пустым, если есть вложения.
- `replyToMessageId` опционален.

```json
{ "messageId": "1001" }
```

### `GET /api/v1/chats/{chatId}/messages?limit=&beforeId=`

Пагинация **назад** (история):

- без `beforeId` — самые новые,
- с `beforeId` — сообщения старше этого id,
- ответ: `messages` + `hasMore`.

Порядок в текущем клиенте: новые ближе к началу массива первой страницы (проверяйте и сортируйте UI-ом).

```json
{
  "messages": [ /* Message */ ],
  "hasMore": true
}
```

### `PATCH /api/v1/messages/{messageId}`

```json
{ "messageId": 1001, "text": "edited" }
```

### `DELETE /api/v1/messages/{messageId}`

### `POST /api/v1/messages/{messageId}:pin`

```json
{ "messageId": 1001, "isPinned": true }
```

### `POST /api/v1/chats/{chatId}:pin`

```json
{ "chatId": 10, "isPinned": true }
```

Pin **чата в сайдбаре** для текущего пользователя (не путать с pin сообщения).

### `POST /api/v1/chats/{chatId}:read`

```json
{ "chatId": 10, "messageId": 1001 }
```

Отметить прочитанным до `messageId` включительно → уменьшает `unreadCount`.

### `GET /api/v1/chats:unread?chatIds=1&chatIds=2`

(или как Envoy развернёт repeated query). Ответ:

```json
{ "counts": { "1": "3", "2": "0" } }
```

---

## 7. Media API

### Модель `File`

```ts
type MediaFile = {
  fileId: string
  ownerId?: number | string
  mime?: string
  sizeBytes?: number | string
  status?: string       // "pending" | "ready" | …
  getUrl?: string       // presigned GET URL
  createdAt?: string
}
```

### Лимиты (дефолт сервера)

- Max size: **25 MiB** (`MAX_UPLOAD_BYTES=26214400`), если не изменено.
- MIME prefixes (дефолт): `image/`, `video/`, `audio/`, `application/pdf`, `text/plain`.

### Аватар / вложение — один и тот же flow

`avatarFileId` у user/chat и `attachmentIds` у message — это **`fileId`** из Media, не URL.

### Upload flow (обязательный)

1. **`POST /api/v1/media/uploads`**

```json
{ "filename": "photo.jpg", "mime": "image/jpeg", "sizeBytes": 12345 }
```

```json
{ "fileId": "uuid…", "putUrl": "https://…presigned…" }
```

2. **`PUT putUrl`** — бинарное тело файла  
   - Обычно **без** `Authorization` (подпись в query).  
   - `Content-Type` = тот же `mime`, что в init (если требует S3).

3. **`POST /api/v1/media/uploads/{fileId}:complete`**

```json
{ "fileId": "uuid…" }
```

→ `File` со `status: "ready"` и `getUrl`.

4. Использовать `fileId`:
   - сообщение: `attachmentIds: [fileId]`,
   - профиль: `PATCH /users/me` `{ "avatarFileId": fileId }`,
   - группа: `PATCH /chats/{id}` `{ "avatarFileId": fileId }`.

### `GET /api/v1/media/files/{fileId}`

Метаданные + актуальный `getUrl` (если истекает — перезапросите).

### `DELETE /api/v1/media/files/{fileId}`

### Отображение картинок

- Предпочитайте `getUrl` из `GetFile` / `CompleteUpload`.
- На проде public endpoint может быть path-style через origin (`/messanger/...`); `getUrl` уже корректный для браузера.

---

## 8. Search API

### `GET /api/v1/search/messages?query=&chatId=&cursor=&limit=`

- `chatId` опционален (поиск в одном чате или глобально по доступным).
- Ответ:

```json
{
  "hits": [
    {
      "messageId": "1",
      "chatId": "10",
      "senderId": "2",
      "text": "…",
      "score": 1.2
    }
  ],
  "nextCursor": ""
}
```

### `GET /api/v1/search/chats?query=&cursor=&limit=`

```json
{
  "hits": [
    { "chatId": "10", "name": "Team", "description": "…", "score": 1.0 }
  ],
  "nextCursor": ""
}
```

**Люди** ищутся через `GET /api/v1/users:search`, не через search-сервис.

Рекомендуемый unified search UI:

1. `users:search`
2. `search/chats`
3. `search/messages`

---

## 9. Notify API (push, опционально для web)

### `POST /api/v1/notify/devices`

```json
{ "platform": "web" | "ios" | "android", "pushToken": "…" }
```

### `DELETE /api/v1/notify/devices/{pushToken}`

---

## 10. WebSocket Realtime

### Подключение

- URL: `wss://<host>/ws?token=<accessToken>` (также `/v1/ws`).
- При успехе сервер ставит пользователя online и подписывает на fanout по его chat membership.
- Ping от сервера каждые ~30s; клиент отвечает pong (браузер делает сам).
- Read deadline ~60s без pong → disconnect.
- Rate limit входящих сообщений (сервер): при превышении `{ "type":"error","code":"rate_limited" }`, затем close.

Reconnect: exponential backoff (текущий SPA: `min(10s, 500ms * 2^attempt)`).

### Client → Server

Все JSON text frames.

#### Typing

```json
{ "type": "typing", "chat_id": 10 }
```

Сервер сохраняет typing TTL (~5s) и рассылает в чат.

#### Presence subscribe

```json
{ "type": "presence.subscribe", "user_ids": [2, 3, 4] }
```

Максимум **100** id за запрос. Ответ только запросившему:

```json
{
  "type": "presence",
  "users": { "2": true, "3": false }
}
```

Ключи — **строки** user id.

#### Ping

```json
{ "type": "ping" }
```

```json
{ "type": "pong" }
```

### Server → Client

#### Typing (fanout в чат)

```json
{ "type": "typing", "chat_id": 10, "user_id": 2 }
```

#### Chat realtime envelope

```json
{
  "type": "chat.realtime",
  "event": "message_created",
  "event_id": "uuid",
  "chat_id": 10,
  "actor_id": 2,
  "payload": "<base64 protobuf optional>",
  "message": {
    "message_id": 1001,
    "sender_id": 2,
    "text": "hi",
    "attachment_ids": [],
    "chat_id": 10,
    "send_at": "2026-10-02T20:00:00Z"
  }
}
```

| `event` | Поля `message` / смысл |
|---------|-------------------------|
| `message_created` | `message_id`, `sender_id`, `text`, `attachment_ids`, `send_at` |
| `message_edited` | `message_id`, `text`, `updated_at` |
| `message_deleted` | `message_id` |
| `message_pinned` | `message_id`, `is_pinned` |
| `receipt_read` | (детали в protobuf payload; обновите unread/UI) |
| `member_changed` | участник добавлен/удалён/роль; **invalidate** chat/members |

Поля envelope в **snake_case**.

Рекомендации UI:

- На `message_created`: upsert в кэш сообщений; обновить preview/order чата; если не своё — `unreadCount++`.
- На edit/delete/pin: патч кэша.
- На `member_changed` / неизвестное: invalidate `chats` / `messages` / `getChat`.
- После открытия чата: `markRead` + `presence.subscribe` по `participantIds` / `peerUserId`.
- Typing: throttle send (~2–3s); показывать индикатор с авто-скрытием по TTL.

---

## 11. Рекомендуемые пользовательские сценарии

### Bootstrap после логина

1. `GET /users/me`
2. `GET /chats?limit=50`
3. Открыть WS
4. Собрать peer/member ids → `presence.subscribe`
5. Для аватаров: батч `GET /users/{id}` + `GET /media/files/{avatarFileId}` (с кэшем)

### Новый DM

1. Search users → выбрать
2. `POST /chats:direct` `{ peerUserId }`
3. Открыть чат / `GET /chats/{id}/messages`

### Новая группа

1. Имя + ≥1 участник
2. `POST /chats` с `chatInfo`
3. Опционально upload avatar → `PATCH /chats/{id}`

### Отправка с фото

1. Media init → PUT → complete
2. `POST .../messages` с `attachmentIds` (+ optional text/reply)
3. Оптимистичный UI + сверка по WS `message_created` / ответу `messageId`

### Jump to search hit

1. Открыть `chatId`
2. Подгрузить историю `beforeId` пока не появится `messageId` (или отдельный fetch вокруг id, если добавите)
3. Подсветить сообщение

---

## 12. Полная карта HTTP-эндпоинтов

| Method | Path | Auth | Сервис |
|--------|------|------|--------|
| POST | `/api/v1/auth/login` | no | auth |
| POST | `/api/v1/auth/refresh` | no | auth |
| POST | `/api/v1/auth/access` | no | auth |
| GET | `/api/v1/auth/validate` | yes | auth |
| POST | `/api/v1/auth/logout` | yes* | auth |
| POST | `/api/v1/users` | no | user |
| GET | `/api/v1/users/me` | yes | user |
| PATCH | `/api/v1/users/me` | yes | user |
| POST | `/api/v1/users/me/password` | yes | user |
| DELETE | `/api/v1/users/me` | yes | user |
| GET | `/api/v1/users/{id}` | yes | user |
| GET | `/api/v1/users/by-username/{username}` | yes | user |
| GET | `/api/v1/users:search` | yes | user |
| POST | `/api/v1/chats` | yes | chat |
| POST | `/api/v1/chats:direct` | yes | chat |
| GET | `/api/v1/chats` | yes | chat |
| GET | `/api/v1/chats:ids` | yes | chat |
| GET | `/api/v1/chats:unread` | yes | chat |
| GET | `/api/v1/chats/{chatId}` | yes | chat |
| PATCH | `/api/v1/chats/{chatId}` | yes | chat |
| DELETE | `/api/v1/chats/{chatId}` | yes | chat |
| POST | `/api/v1/chats/{chatId}/members` | yes | chat |
| DELETE | `/api/v1/chats/{chatId}/members/{userId}` | yes | chat |
| PATCH | `/api/v1/chats/{chatId}/members/{userId}` | yes | chat |
| POST | `/api/v1/chats/{chatId}/messages` | yes | chat |
| GET | `/api/v1/chats/{chatId}/messages` | yes | chat |
| POST | `/api/v1/chats/{chatId}:read` | yes | chat |
| POST | `/api/v1/chats/{chatId}:pin` | yes | chat |
| PATCH | `/api/v1/messages/{messageId}` | yes | chat |
| DELETE | `/api/v1/messages/{messageId}` | yes | chat |
| POST | `/api/v1/messages/{messageId}:pin` | yes | chat |
| POST | `/api/v1/media/uploads` | yes | media |
| POST | `/api/v1/media/uploads/{fileId}:complete` | yes | media |
| GET | `/api/v1/media/files/{fileId}` | yes | media |
| DELETE | `/api/v1/media/files/{fileId}` | yes | media |
| GET | `/api/v1/search/messages` | yes | search |
| GET | `/api/v1/search/chats` | yes | search |
| POST | `/api/v1/notify/devices` | yes | notify |
| DELETE | `/api/v1/notify/devices/{pushToken}` | yes | notify |
| WS | `/ws` or `/v1/ws` | token | gateway |

\*logout принимает refresh в body; Bearer желателен.

---

## 13. CORS / Origin

Gateway проверяет `Origin` против `WS_ALLOWED_ORIGINS` (прод: `https://chat.arbuz-arbuz.site`).  
REST идёт same-origin через web nginx → CORS обычно не нужен для SPA на том же хосте.

---

## 14. Что не публично

- gRPC `UserService.ValidateCredentials` — только internal.
- Прямой доступ к Postgres/Redis/Kafka/MinIO admin.
- Envoy admin (`9901`), metrics порты сервисов.

---

## 15. Proto source of truth

| Domain | File |
|--------|------|
| Auth | `shared/proto/auth/v1/auth.proto` |
| User | `shared/proto/user/v1/user.proto` |
| Chat | `shared/proto/chat/v1/chat.proto` |
| Media | `shared/proto/media/v1/media.proto` |
| Search | `shared/proto/search/v1/search.proto` |
| Notify | `shared/proto/notify/v1/notify.proto` |
| Realtime events | `shared/proto/events/v1/events.proto` |

При расхождении документа и proto — **побеждает proto** + поведение Envoy (`print_options` / HTTP annotations).

---

## 16. Чеклист готовности фронта

- [ ] Login / register / logout / silent refresh
- [ ] Me + profile edit + avatar upload
- [ ] Chat list (pin, unread, preview, DIRECT title = peer name)
- [ ] Open DM / create group
- [ ] Message list pagination + send/edit/delete/pin/reply
- [ ] Attachments upload + image preview
- [ ] Mark read on open/scroll
- [ ] WS connect + reconnect + typing + presence
- [ ] Apply `chat.realtime` to caches
- [ ] Unified search (users/chats/messages) + jump
- [ ] Group info: members add/remove (admin-only UI)
- [ ] Normalize int64 strings & dual enum forms
- [ ] Handle 401 → refresh → retry
