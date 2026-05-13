# olcRTC API

REST API для управления туннельными каналами.

## Аутентификация

Все запросы требуют заголовок:

```
Authorization: Bearer <OLCRTC_MASTER_KEY>
```

---

## Endpoints

| Метод    | Путь                            | Описание                 |
|----------|---------------------------------|--------------------------|
| `POST`   | `/api/v1/channels`              | Создать канал            |
| `GET`    | `/api/v1/channels`              | Список каналов           |
| `GET`    | `/api/v1/channels/{id}`         | Получить канал           |
| `PUT`    | `/api/v1/channels/{id}`         | Обновить канал           |
| `DELETE` | `/api/v1/channels/{id}`         | Удалить канал            |
| `POST`   | `/api/v1/channels/{id}/renew`   | Продлить срок действия   |
| `GET`    | `/api/v1/status`                | Статус сервера           |

---

## Провайдеры (carrier)

| Carrier    | Описание                     | `room_id`           |
|------------|------------------------------|---------------------|
| `jazz`     | SaluteJazz (SberDevices)     | Автогенерация       |
| `wbstream` | WB Stream (Wildberries)      | **Обязателен**      |
| `telemost` | Yandex Telemost              | **Обязателен**      |

### jazz

Комната создаётся автоматически через API SaluteJazz. Поле `room_id` можно не передавать.

### wbstream

Комната **не** создаётся автоматически. Необходимо передать существующий `room_id`.

### telemost

Комната **не** создаётся автоматически. Необходимо передать `room_id` (идентификатор конференции из URL `https://telemost.yandex.ru/j/{room_id}`).

---

## Транспорты (transport)

| Transport      | Описание                             | Специфичные параметры |
|----------------|--------------------------------------|-----------------------|
| `datachannel`  | WebRTC DataChannel (прямой байтовый) | Нет                   |
| `vp8channel`   | KCP поверх VP8 видеофреймов          | Опциональные          |
| `seichannel`   | Данные в SEI-сообщениях H.264        | Опциональные          |
| `videochannel` | Визуальное кодирование (QR/tile)     | **Обязательные**      |

---

## Матрица: carrier + transport

Все комбинации carrier/transport поддерживаются:

| | `datachannel` | `vp8channel` | `seichannel` | `videochannel` |
|---|:---:|:---:|:---:|:---:|
| **jazz** | + | + | + | + |
| **wbstream** | + | + | + | + |
| **telemost** | + | + | + | + |

---

## Параметры `transport_config`

### datachannel

Дополнительные параметры не требуются. Самый простой транспорт.

```json
{
  "carrier": "jazz",
  "transport": "datachannel",
  "client_id": "my-client"
}
```

### vp8channel

Все параметры опциональны. Если не указаны — применяются значения по умолчанию.

| Поле              | Тип  | По умолчанию | Описание                           |
|-------------------|------|:------------:|------------------------------------|
| `vp8_fps`         | int  | 25           | Частота кадров VP8                 |
| `vp8_batch_size`  | int  | 1            | Кадров за один тик                 |

```json
{
  "carrier": "wbstream",
  "transport": "vp8channel",
  "client_id": "my-client",
  "room_id": "existing-room-id",
  "transport_config": {
    "vp8_fps": 25,
    "vp8_batch_size": 1
  }
}
```

### seichannel

Все параметры опциональны. Если не указаны — применяются значения по умолчанию.

| Поле                 | Тип  | По умолчанию | Описание                           |
|----------------------|------|:------------:|------------------------------------|
| `sei_fps`            | int  | 20           | Частота кадров SEI                 |
| `sei_batch_size`     | int  | 1            | Кадров за один тик                 |
| `sei_fragment_size`  | int  | 900          | Размер фрагмента (байт)           |
| `sei_ack_timeout_ms` | int  | 3000         | Таймаут ACK (мс)                   |

```json
{
  "carrier": "telemost",
  "transport": "seichannel",
  "client_id": "my-client",
  "room_id": "conference-id",
  "transport_config": {
    "sei_fps": 20,
    "sei_batch_size": 1,
    "sei_fragment_size": 900,
    "sei_ack_timeout_ms": 3000
  }
}
```

### videochannel

**Обязательные параметры** (без них канал не создастся):

| Поле            | Тип    | Описание                              |
|-----------------|--------|---------------------------------------|
| `video_width`   | int    | Ширина видео (пикс.)                 |
| `video_height`  | int    | Высота видео (пикс.)                 |
| `video_fps`     | int    | Частота кадров                        |
| `video_bitrate` | string | Битрейт (`"1M"`, `"500K"`)           |
| `video_hw`      | string | Аппаратное ускорение (`"none"`, `"nvenc"`) |

**Опциональные параметры:**

| Поле               | Тип    | По умолчанию | Описание                                          |
|--------------------|--------|:------------:|---------------------------------------------------|
| `video_codec`      | string | `"qrcode"`   | Визуальный кодек: `"qrcode"` или `"tile"`         |
| `video_qr_size`    | int    | 256          | Размер QR-фрагмента                               |
| `video_qr_recovery`| string | `"low"`      | Уровень коррекции: `low`, `medium`, `high`, `highest` |
| `video_tile_module`| int    | 4            | Размер тайла в пикселях (1..270)                  |
| `video_tile_rs`    | int    | 20           | Процент чётности Reed-Solomon (0..200)            |

> При использовании `video_codec: "tile"` размеры должны быть `1080x1080`.

```json
{
  "carrier": "jazz",
  "transport": "videochannel",
  "client_id": "my-client",
  "transport_config": {
    "video_width": 640,
    "video_height": 480,
    "video_fps": 30,
    "video_bitrate": "1M",
    "video_hw": "none",
    "video_codec": "qrcode"
  }
}
```

---

## Общие параметры создания канала

| Поле              | Тип    | Обязательно      | По умолчанию     | Описание                              |
|-------------------|--------|:----------------:|:----------------:|---------------------------------------|
| `carrier`         | string | Да               | —                | Провайдер: `jazz`, `wbstream`, `telemost` |
| `transport`       | string | Да               | —                | Транспорт: `datachannel`, `vp8channel`, `seichannel`, `videochannel` |
| `client_id`       | string | Да               | —                | Идентификатор клиента                 |
| `room_id`         | string | Для wbstream, telemost | Автогенерация (jazz) | Идентификатор комнаты            |
| `link`            | string | Нет              | `"direct"`       | Тип соединения                        |
| `dns_server`      | string | Нет              | `"1.1.1.1:53"`   | DNS-сервер                            |
| `socks_proxy_addr`| string | Нет              | —                | Адрес SOCKS5 прокси                   |
| `socks_proxy_port`| int    | Нет              | 0                | Порт SOCKS5 прокси                    |
| `ttl_days`        | int    | Нет              | 30               | Время жизни канала (дни)              |
| `transport_config`| object | Зависит от транспорта | —           | Параметры транспорта (см. выше)       |

---

## Примеры запросов

### Минимальный канал (jazz + datachannel)

```bash
curl -X POST http://localhost:8080/api/v1/channels \
  -H "Authorization: Bearer $OLCRTC_MASTER_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "carrier": "jazz",
    "transport": "datachannel",
    "client_id": "user-1"
  }'
```

Комната и ключ шифрования генерируются автоматически.

### Канал wbstream + vp8channel

```bash
curl -X POST http://localhost:8080/api/v1/channels \
  -H "Authorization: Bearer $OLCRTC_MASTER_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "carrier": "wbstream",
    "transport": "vp8channel",
    "client_id": "user-1",
    "room_id": "existing-wbstream-room-id",
    "transport_config": {
      "vp8_fps": 25,
      "vp8_batch_size": 2
    }
  }'
```

### Канал telemost + seichannel

```bash
curl -X POST http://localhost:8080/api/v1/channels \
  -H "Authorization: Bearer $OLCRTC_MASTER_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "carrier": "telemost",
    "transport": "seichannel",
    "client_id": "user-1",
    "room_id": "conference-id-from-url"
  }'
```

### Канал с videochannel (обязательные параметры)

```bash
curl -X POST http://localhost:8080/api/v1/channels \
  -H "Authorization: Bearer $OLCRTC_MASTER_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "carrier": "jazz",
    "transport": "videochannel",
    "client_id": "user-1",
    "transport_config": {
      "video_width": 640,
      "video_height": 480,
      "video_fps": 30,
      "video_bitrate": "1M",
      "video_hw": "none"
    }
  }'
```

### Продление канала

```bash
curl -X POST http://localhost:8080/api/v1/channels/{id}/renew \
  -H "Authorization: Bearer $OLCRTC_MASTER_KEY" \
  -H "Content-Type: application/json" \
  -d '{"ttl_days": 60}'
```

### Статус сервера

```bash
curl http://localhost:8080/api/v1/status \
  -H "Authorization: Bearer $OLCRTC_MASTER_KEY"
```

**Ответ:**

```json
{
  "active_channels": 3,
  "max_channels": 10,
  "available_slots": 7,
  "can_create": true,
  "channels": [
    {
      "id": "uuid",
      "carrier": "jazz",
      "transport": "datachannel",
      "status": "running"
    }
  ]
}
```

---

## Коды ошибок

| Код | Описание                                           |
|:---:|----------------------------------------------------|
| 400 | Невалидный запрос (отсутствует обязательное поле, неизвестный carrier/transport) |
| 401 | Неверный или отсутствующий токен авторизации        |
| 404 | Канал не найден                                    |
| 409 | Достигнут лимит каналов                            |
| 500 | Внутренняя ошибка сервера                          |

---

## Переменные окружения

| Переменная            | Обязательна | По умолчанию              | Описание                  |
|-----------------------|:-----------:|---------------------------|---------------------------|
| `OLCRTC_MASTER_KEY`   | Да          | —                         | Bearer-токен для API      |
| `OLCRTC_MAX_CHANNELS` | Нет         | `10`                      | Лимит каналов             |
| `OLCRTC_DB_PATH`      | Нет         | `data/channels.db`        | Путь к SQLite БД          |
| `OLCRTC_API_LISTEN`   | Нет         | `:8080`                   | Адрес HTTP-сервера        |

---

## Запуск

```bash
# Из Docker
docker run -e OLCRTC_MASTER_KEY=secret burgerbot/olcrtc-apified -mode api

# Docker Compose
docker compose -f docker-compose.api.yml up
```
