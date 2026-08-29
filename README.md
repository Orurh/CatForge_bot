# CatForge Bot

CatForge — социальная Telegram-игра, где у каждого участника есть один кот с
характером. Кот тренируется, дерётся с котами друзей и участвует в событиях Двора.
AI добавляет личность и редкие комментарии,
но не определяет игровые результаты.

Игра работает прямо в private chat, group и supergroup. Mini App в текущий
roadmap не входит.

## Текущий MVP

- создание, профиль, переименование и удаление кота;
- одна команда `/train`, которая сразу проводит тренировку;
- энергия `0..100`, минимальная стоимость 25 и расход
  `clamp(energy - 10, 25, 90)`;
- Yard и асинхронное событие «Машина с рыбой»;
- автоматическая очередь арены и детерминированный C++-бой по `/fight`;
- `/askcat`, reply-команда `/cat`, personality, humor и autospeak;
- редкие автономные реплики с лимитами Двора;
- `/week` как скрытая secondary-команда итогов Двора.

`/fight` работает как toggle. Первый кот встаёт в очередь Двора, второй
автоматически начинает с ним бой, а повторная команда ждущего кота убирает его с
арены. Ожидание истекает через 15 минут.

Expedition, Collection, Bestiary и Equipment сохранены как legacy-код, но
скрыты из command menu, Main Menu и активного профиля. `/bind` и `/unbind`
больше не используются: результат действия остаётся в том чате, где оно было
запущено.

## Telegram-команды

В private chat доступно всё персональное управление:

```text
/start
/profile
/train
/name [имя]
/reset
/askcat вопрос
/cat
/autospeak on|off
/humor normal|bold
```

В группе остаются публичные игровые действия:

```text
/start       # нейтральная кнопка открыть private chat
/profile     # публичная read-only карточка без настроек
/train
/name имя   # переименовать кота прямо в группе
/askcat
/cat
/yard
/event
/fight
```

Администраторы группы получают отдельный command scope:

```text
/yardsettings
/quiet 24h|off
```

Экраны создания, юмора, автореплик и удаления кота никогда не рендерятся в группе: вместо них бот
публикует только нейтральную deep-link кнопку. Owner-bound callbacks дополнительно блокируют старые карточки.
В группе имя меняется одной командой: `/name@TryToGreat_bot НовоеИмя`. Выбор породы, humor, autospeak и
удаление кота остаются private-only.

## Процедурные реплики

Все игровые реплики, которые не генерирует LLM, вынесены из Go-кода в
[`internal/gamecontent/texts`](internal/gamecontent/texts):

- `training.json` — тренировки и реакции traits;
- `fight.json` — старт, результат, close fight, upset, revenge и лимиты;
- `yard_events.json` — процедурные истории событий Двора;
- `ai_fallback.json` — тексты на случай отключённого/недоступного AI;
- `legacy_expedition.json` — замороженные Expedition narratives;
- `ui.json` — активные command descriptions, кнопки и короткие подсказки.

Чтобы расширить набор реплик, добавьте строку в массив нужного ключа. Можно
использовать шаблоны `{{.CatName}}`, `{{.WinnerName}}` и другие поля, уже
показанные в файлах. Каталог проверяет JSON, пустые варианты, дубликаты ключей
и синтаксис шаблонов:

```bash
go test ./internal/gamecontent
```

Контент встраивается в бинарник, поэтому после редактирования нужен rebuild.
Подробности находятся в
[`internal/gamecontent/texts/README.md`](internal/gamecontent/texts/README.md).

## Запуск для разработки

1. Скопируйте `.env.example` в `.env` и заполните Telegram token.
2. Запустите стек:

   ```bash
   docker compose -f docker-compose.dev.yml up -d --build
   ```

3. Проверьте сервисы:

   ```bash
   docker compose -f docker-compose.dev.yml ps
   curl -i http://127.0.0.1:8080/healthz
   docker compose -f docker-compose.dev.yml logs --since=2m bot game-engine
   ```

В dev используется Telegram long polling. Нормальный лог содержит
`telegram long polling enabled`. Production использует webhook с публичным
HTTPS URL и `TELEGRAM_WEBHOOK_SECRET`.

Остановка без удаления PostgreSQL volume:

```bash
docker compose -f docker-compose.dev.yml stop
```

Не добавляйте `-v` к `docker compose down`, если игровой прогресс нужно
сохранить.

## Проверки

```bash
go test -race ./...
go vet ./...

cmake -S engine_cpp -B engine_cpp/build -DCMAKE_BUILD_TYPE=Debug
cmake --build engine_cpp/build --parallel
ctest --test-dir engine_cpp/build --output-on-failure
```

После изменения Protobuf-контракта:

```bash
./scripts/generate_proto.sh
```

## Архитектура

- `cmd/bot` — запуск приложения и фоновых workers;
- `internal/app` — application services и GameEvent layer;
- `internal/domain` — игровые типы и чистые правила;
- `internal/gamecontent` — валидируемый каталог процедурных текстов;
- `internal/gameengine` — Go-контракт и gRPC-клиент;
- `engine_cpp` — авторитетный C++20 game engine;
- `api/gameengine/v1` — Protobuf-контракт Go/C++;
- `internal/ai` — provider-independent AI Gateway и safe fallback;
- `internal/repo/postgres` — PostgreSQL persistence;
- `internal/transport/telegram` — routing, callbacks, Telegram API и views;
- `migrations` — миграции Goose.

C++-движок определяет игровые факты. Go загружает состояние, вызывает движок,
сохраняет результат через optimistic `state_version`, публикует GameEvent и
формирует Telegram UX. AI получает только готовые факты и не может менять XP,
награды, outcome или relationships.

Текущие версии: `rules_version=8`, `content_version=2`.

Временный dev-tuning восстанавливает 1 энергию в секунду. Перед production
нужно вернуть целевое значение: примерно 1 энергия за 8 минут.

## AI provider

По умолчанию работает procedural provider из редактируемого каталога. Для
OpenAI Responses API задайте:

```dotenv
AI_PROVIDER=openai
AI_API_KEY=...
AI_MODEL=...
AI_BASE_URL=https://api.openai.com/v1
AI_TIMEOUT=5s
```

При timeout или ошибке provider бот использует procedural fallback. Запросы
отправляются с `store: false`; API key и полный prompt не логируются. Privacy
Mode Telegram остаётся включённым: бот получает команды, callbacks, replies на
свои сообщения и явные игровые взаимодействия, но не читает весь обычный чат.
