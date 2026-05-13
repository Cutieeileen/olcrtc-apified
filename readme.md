<div align="center">

<img src="https://github.com/openlibrecommunity/material/blob/master/olcrtc.png" width="250" height="250">

# olcrtc-apified

![License](https://img.shields.io/badge/license-WTFPL-0D1117?style=flat-square&logo=open-source-initiative&logoColor=green&labelColor=0D1117)
![Golang](https://img.shields.io/badge/-Golang-0D1117?style=flat-square&logo=go&logoColor=00A7D0)
![Docker](https://img.shields.io/badge/-Docker-0D1117?style=flat-square&logo=docker&logoColor=2496ED)

Multi-tenant форк [olcRTC](https://github.com/openlibrecommunity/olcrtc) с REST API для управления множеством туннелей с одного сервера.

[English version](README.en.md)

</div>

## Возможности

- **Многоканальное управление** — один сервер обслуживает множество туннелей одновременно через REST API
- **Автогенерация ключей** — уникальный 256-битный ключ шифрования на каждый канал (ChaCha20-Poly1305)
- **Автоматическое создание комнат** — комнаты для Jazz создаются автоматически
- **Срок действия каналов** — настраиваемый TTL с автоматической очисткой просроченных каналов
- **Персистентное хранилище** — база данных SQLite, каналы переживают перезапуски
- **Авторизация по Bearer-токену** — единый мастер-ключ защищает все API-эндпоинты
- **Лимит каналов** — настраиваемое ограничение на количество одновременных каналов
- **Полное редактирование каналов** — смена carrier, transport, ключа, комнаты с автоматическим перезапуском туннеля
- **Docker-образ** — мультиархитектурный образ на Docker Hub (`linux/amd64`, `linux/arm64`)

## API-эндпоинты

| Метод | Путь | Описание |
|-------|------|----------|
| `POST` | `/api/v1/channels` | Создать канал |
| `GET` | `/api/v1/channels` | Список всех каналов |
| `GET` | `/api/v1/channels/{id}` | Получить канал по ID |
| `PUT` | `/api/v1/channels/{id}` | Обновить канал (перезапуск туннеля) |
| `DELETE` | `/api/v1/channels/{id}` | Удалить канал |
| `POST` | `/api/v1/channels/{id}/renew` | Продлить срок действия |
| `GET` | `/api/v1/status` | Статус сервера |

Все эндпоинты требуют заголовок `Authorization: Bearer <OLCRTC_MASTER_KEY>`.

Подробная документация: **[API.md](API.md)** — все параметры для каждой комбинации carrier/transport.

## Быстрый старт (Docker)

### Docker Compose

```bash
export OLCRTC_MASTER_KEY="ваш-секретный-ключ"
docker compose -f docker-compose.api.yml up -d
```

### Docker Run

```bash
docker run -d \
  --name olcrtc-apified \
  -p 8080:8080 \
  -e OLCRTC_MODE=api \
  -e OLCRTC_MASTER_KEY="ваш-секретный-ключ" \
  -e OLCRTC_MAX_CHANNELS=10 \
  -v olcrtc-data:/var/lib/olcrtc \
  burgerbot/olcrtc-apified
```

### Проверка

```bash
curl -s -H "Authorization: Bearer ваш-секретный-ключ" \
  http://localhost:8080/api/v1/status
```

## Переменные окружения

| Переменная | Обязательная | По умолчанию | Описание |
|------------|:------------:|--------------|----------|
| `OLCRTC_MASTER_KEY` | да | - | Bearer-токен для авторизации |
| `OLCRTC_MAX_CHANNELS` | нет | `10` | Максимум одновременных каналов |
| `OLCRTC_DB_PATH` | нет | `/var/lib/olcrtc/channels.db` | Путь к базе данных SQLite |
| `OLCRTC_API_LISTEN` | нет | `:8080` | Адрес HTTP-сервера |
| `OLCRTC_DNS` | нет | `1.1.1.1:53` | DNS-сервер по умолчанию |
| `OLCRTC_DEBUG` | нет | `false` | Подробное логирование |

## Примеры использования API

### Создание канала (jazz — комната генерируется автоматически)

```bash
curl -X POST http://localhost:8080/api/v1/channels \
  -H "Authorization: Bearer $OLCRTC_MASTER_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "carrier": "jazz",
    "transport": "datachannel",
    "client_id": "user-1",
    "ttl_days": 30
  }'
```

### Создание канала (wbstream — room_id обязателен)

```bash
curl -X POST http://localhost:8080/api/v1/channels \
  -H "Authorization: Bearer $OLCRTC_MASTER_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "carrier": "wbstream",
    "transport": "datachannel",
    "client_id": "user-1",
    "room_id": "existing-room-id",
    "ttl_days": 30
  }'
```

### Продление канала

```bash
curl -X POST http://localhost:8080/api/v1/channels/<id>/renew \
  -H "Authorization: Bearer $OLCRTC_MASTER_KEY" \
  -H "Content-Type: application/json" \
  -d '{"ttl_days": 60}'
```

### Удаление канала

```bash
curl -X DELETE http://localhost:8080/api/v1/channels/<id> \
  -H "Authorization: Bearer $OLCRTC_MASTER_KEY"
```

## Сборка из исходников

```bash
# установить mage
go install github.com/magefile/mage@latest

# собрать CLI
mage buildCLI

# кросс-компиляция для linux / windows / darwin
mage cross

# android aar через gomobile
mage mobile

# контейнер
mage docker

# тесты
mage test

# линтер
mage lint
```

## Сообщество

- Telegram: [@openlibrecommunity](https://t.me/openlibrecommunity)
- Android-клиент: [alananisimov/olcbox](https://github.com/alananisimov/olcbox)
- Баги и предложения: [GitHub Issues](https://github.com/Cutieeileen/olcrtc-apified/issues)

---

<div align="center">

### Credits

На основе [olcRTC](https://github.com/openlibrecommunity/olcrtc) от **zarazaex**

Telegram: [zarazaex](https://t.me/zarazaexe) | Email: [zarazaex@tuta.io](mailto:zarazaex@tuta.io) | Site: [zarazaex.xyz](https://zarazaex.xyz)

Made for: [olcNG](https://github.com/zarazaex69/olcng)

</div>
