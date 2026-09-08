# Мониторинг CatForge

## Проверено 7 сентября 2026

Локальный стек запущен. Все 9 scrape jobs UP; `/readyz`, PostgreSQL, C++ и
каталог резервных копий проверены. Grafana загрузила дашборд, оба datasource
здоровы; реальные логи бота получены через Alloy → Loki → Grafana. Promtool
проверил 21 правило и семь сценариев; конфигурации Alertmanager, Loki и Alloy
прошли официальные валидаторы. Go race tests и vet проходят.

По выбранному режиму предупреждения остаются только в Grafana и Alertmanager;
отправка в Telegram не включена. Повторная проверка запущенного стека:
`./scripts/verify_monitoring.sh`.

## Стек

- Prometheus собирает метрики каждые 15 секунд, хранит до 15 дней / 5 GB.
- Grafana содержит готовый дашборд `CatForge — Operations` и Explore для логов.
- Loki хранит логи 7 дней; Alloy читает Docker stdout/stderr только контейнеров
  текущего Compose-проекта с меткой `catforge.logs=true`.
- Alertmanager группирует тревоги и хранит silences. По умолчанию тревоги видны
  локально; внешняя отправка не включена до настройки получателя.
- Node Exporter измеряет CPU/RAM/диски Linux-хоста; Blackbox проверяет `/readyz`.

Это один локальный стек. Для поиска логов используется Loki, поэтому отдельные
Elasticsearch, Logstash и Kibana здесь не требуются. Метрики и логи открываются
из одного интерфейса Grafana.

## Запуск

```bash
./scripts/setup_monitoring.sh
./scripts/check_monitoring.sh
MONITORING_BACKUP_GID=$(stat -c %g backups) \
  docker compose -f docker-compose.dev.yml -f docker-compose.monitoring.yml \
  up -d --build
```

Для production замените `docker-compose.dev.yml` на `docker-compose.yml`.
При последующих обновлениях продолжайте передавать оба файла: без overlay
у бота исчезнут сетевой metrics listener и доступ к каталогу бэкапов.
Скрипт setup не перезаписывает существующий пароль. Пароль хранится в
`.monitoring-secrets/grafana_password`; каталог закрыт правами 700 и исключён из
Git, Docker build context и текстового сборника кода.

| Интерфейс | Адрес |
|---|---|
| Grafana | http://127.0.0.1:3000/d/catforge-overview |
| Prometheus | http://127.0.0.1:9090 |
| Alertmanager | http://127.0.0.1:9093 |

Вход Grafana: `admin`, пароль из файла выше. Анонимный доступ выключен.
Все опубликованные порты мониторинга привязаны к localhost. Для удалённого
сервера используйте SSH-туннель: `ssh -L 3000:127.0.0.1:3000 user@server`.
Loki, Alloy, exporters и metrics listener бота не публикуются наружу.
В обычном запуске без overlay metrics listener слушает `127.0.0.1:9091`;
с overlay — `0.0.0.0:9091` внутри Docker-сети. Публичный HTTP-сервер не содержит `/metrics`.

## Что видно

Дашборд показывает доступность компонентов, успешность probes, тревоги, возраст
бэкапа, heartbeat polling/workers, Telegram API ошибки, AI fallback, задержки
C++ RPC, пул БД, CPU/RAM/диски хоста, RSS Go-процесса и логи по сервису.

- `catforge_operations_total`: операции и результат. Telegram `send` объединяет
  методы отправки/редактирования и вспомогательные вызовы Sender, включая API
  ошибки при HTTP 200. `handleUpdate` считает результат транспортного обработчика;
  игровые ограничения внутри UI не являются транспортными ошибками.
- `catforge_operation_duration_seconds`: histogram задержек.
- `catforge_last_success_timestamp_seconds`: последний успешный цикл. Пустой
  успешный `getUpdates` и workers без задач обновляют heartbeat.
- `catforge_dependency_up`: отдельная проверка PostgreSQL и движка каждые 15 секунд.
  `Progress` в статистике RPC включает stateless compatibility probes.
- AI `primary`, `fallback`, `procedural`, `error` различаются. Штатный procedural
  provider не вызывает тревогу отказа внешнего AI.
- Метрики Go runtime/process доступны через официальный Go-клиент Prometheus.
- Ошибки и предупреждения slog считаются отдельно; каждые пять минут бот пишет
  `monitoring heartbeat`, чтобы проверять доставку логов даже при отсутствии игроков.

Имена игроков, Telegram/chat/cat IDs, тексты, prompts, токены и произвольные URL
не используются как labels метрик. Логи содержат существующие диагностические
поля приложения; доступ к Grafana давайте только операторам. Alloy дополнительно
маскирует строки формата bot-token и `sk-*`; это не универсальная очистка персональных данных.

Alloy имеет доступ к Docker socket, Node Exporter — к read-only дереву хоста.
Это доверенные административные компоненты; `:ro` на socket не ограничивает
Docker API до операций чтения. Не публикуйте их служебные интерфейсы.

## Тревоги

Правила находятся в `deploy/monitoring/alerts.yml`. Есть проверки недоступности
сервисов/зависимостей, зависания probes/polling/workers, повторных Telegram/RPC/log
ошибок, высокого AI fallback, медленного движка, насыщения пула БД, диска, памяти,
CPU, свежести бэкапа, потерь логов и ошибок отправки уведомлений.

Тревоги используют выдержки 1–15 минут. Несколько ошибок AI при одной попытке
не вызывают шум: правило требует минимум пять запросов за десять минут.
`alerts_test.yml` проверяет как аварийные сценарии, так и отсутствие ложных тревог
при idle polling, webhook mode и procedural AI.

## Telegram-уведомления

Для отправки нужен отдельный служебный бот и числовой chat ID получателя. Не
используйте игровой token и не добавляйте секреты в tracked YAML.

1. Сохраните token в `.monitoring-secrets/telegram_token`.
2. Скопируйте `deploy/monitoring/alertmanager.telegram.example.yml` в
   `.monitoring-secrets/alertmanager.yml`, замените `chat_id: 0` на реальный ID.
3. Файлы должны читаться процессом Alertmanager внутри контейнера; например,
   права 444 на файлы внутри закрытого каталога 700, как у секрета Grafana.
4. Проверьте конфигурацию и подключите третий overlay:

```bash
docker compose -f docker-compose.dev.yml -f docker-compose.monitoring.yml \
  -f docker-compose.alerts.yml run --rm --no-deps --entrypoint /bin/amtool \
  alertmanager check-config /etc/alertmanager/alertmanager.yml
docker compose -f docker-compose.dev.yml -f docker-compose.monitoring.yml \
  -f docker-compose.alerts.yml up -d alertmanager
```

Включение этого overlay разрешает отправку уведомлений о firing/resolved тревогах
в указанный чат. Реальную доставку нужно проверить отдельным тестовым alert после
выбора получателя; локальная проверка правил не подтверждает доставку в Telegram.

## Бэкапы и ограничения

Мониторинг проверяет время изменения новейшего непустого `.dump` в `backups/`.
Временные файлы и symlinks исключаются. Скрипт `backup_db.sh` публикует архив только
после успешного pg_dump и проверки формата pg_restore. Перезапуск бота не обнуляет
возраст архива. Бот получает только право просматривать каталог через группу;
содержимое файлов 600 ему недоступно.

Метрика не подтверждает успешное восстановление и копию вне сервера. Ежедневный backup
устанавливается скриптом `scripts/setup_backup.sh`; расписание, внешняя копия
и проверка восстановления описаны в BETA_RUNBOOK.md.
Копирование чужого `.dump` в этот каталог или изменение mtime обновляет метрику,
поэтому каталог предназначен только для управляемых резервных копий.

Loki использует локальный диск: 7-дневное хранение ограничивает срок, а не объём.
Docker-логи ротируются по 10 MB × 3 на контейнер. Следите за диском и объёмом Loki.
У контейнеров мониторинга заданы лимиты RAM, суммарно около 2.7 GiB; это потолки,
фактическое потребление зависит от нагрузки. Данные сохраняются в Docker volumes.

Этот стек не сможет отправить сообщение, если целиком выключится его сервер или
пропадёт интернет. Для этого нужен отдельный внешний uptime/dead-man monitor на
другой машине. Webhook mode не использует polling heartbeat; для production
дополнительно проверяйте публичный HTTPS URL снаружи.

## Проверки и диагностика

```bash
./scripts/check_monitoring.sh
GOCACHE=/tmp/catforge-go-cache go test -race ./...
GOCACHE=/tmp/catforge-go-cache go vet ./...
docker compose -f docker-compose.dev.yml -f docker-compose.monitoring.yml ps
curl --fail http://127.0.0.1:9090/-/ready
curl --fail http://127.0.0.1:3000/api/health
```

Explore → Loki: `{application="catforge",service="bot"}`.
Ошибки приложения: `{application="catforge",service="bot"} | json | level="ERROR"`.
В Prometheus → Targets все scrape jobs должны быть UP. В Grafana → Alerting
выберите datasource Alertmanager для silences и текущих внешних тревог.

Платежи: dashboard показывает записи на ручную проверку, ожидающие подтверждения возвраты и возраст благодарностей. Правила `PaymentNeedsReview`, `PaymentThanksDelayed`, `PaymentRefundPending`, `PaymentLedgerProbeFailed` используют состояние ledger, поэтому переживают перезапуск бота. Подробности: [PAYMENTS.md](PAYMENTS.md).
