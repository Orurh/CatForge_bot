# CatForge v3 — Telegram-native план

## 0. Новая формулировка

**CatForge — социальная Telegram-игра, где у каждого участника есть собственный кот с характером. Кот тренируется, дерётся с котами друзей, участвует в общих событиях Двора и иногда сам начинает разговаривать с людьми или другими котами.**

На первом этапе в CatForge существует всего три настоящих игровых действия:

### 🐾 Тренировка

Развитие собственного кота.

### 🎲 События Двора

Совместная асинхронная игра участников группы.

### ⚔️ Драки

Кот против кота внутри группы.

И четвёртая система, которая связывает всё:

### 🗣 Жизнь котов

ИИ иногда комментирует произошедшее, отвечает за хозяина, подкалывает соперника или начинает короткий разговор с другим котом.

---

# 1. Mini App пока полностью убираем из roadmap

На данном этапе отдельное приложение не нужно.

Вся игра должна быть полноценной непосредственно внутри:

* private chat с ботом;
* Telegram group;
* supergroup.

Используем стандартные Telegram-механики:

* команды;
* reply;
* inline buttons;
* ForceReply;
* изображения;
* callback queries;
* обычные сообщения;
* Telegram command scopes.

Если когда-нибудь Telegram UI реально станет ограничением — тогда отдельно рассматриваем Mini App.

Но сейчас:

**Mini App не является планируемой feature.**

---

# 2. Главный принцип интерфейса

Группа — публичное игровое пространство: тренировки, Двор, события, драки и публичная
read-only карточка кота остаются в ней.

Создание кота и все персональные настройки выполняются только в private chat. Telegram Bot API не умеет
показать одному участнику скрытую от остальных inline-карточку внутри группы. Поэтому в группе бот показывает
только нейтральную deep-link кнопку в личный чат, без имени, породы и значений настроек.

---

# 3. Один пользователь — один кот

Кот глобальный для Telegram-пользователя.

Не:

```text
группа A → Барсик
группа B → Мурзик
private → Борис
```

А:

```text
User
  │
  └── Барсик
       ├── Двор друзей
       ├── рабочий Двор
       └── ещё один Двор
```

Барсик везде остаётся Барсиком.

Это критически важно для identity.

---

# 4. Один кот может состоять в нескольких Дворах

Двор принадлежит Telegram group/supergroup.

Один пользователь может иметь:

```text
Барсик
├── Двор «Работа»
├── Двор «Друзья»
└── Двор «Универ»
```

Его:

* уровень;
* порода;
* характер;

общие.

А:

* отношения;
* соперники;
* история событий;

относятся к конкретному Двору.

---

# 5. Убираем Home Chat

Текущие:

```text
/bind
/unbind
```

становятся ненужными.

Сейчас они используются для привязки одной группы, куда может улетать результат личной охоты.

В новой системе это излишняя магия.

Правило намного проще:

### Training в личке

Результат остаётся в личке.

### Training в группе

Результат появляется в этой группе.

### Fight

Принадлежит группе, где произошёл.

### Yard Event

Принадлежит этому Двору.

Никаких скрытых:

> «куда сейчас бот пошлёт сообщение?»

---

# 6. Команды пользователя

Основной набор:

```text
/start
/profile
/train
/name
/reset
/askcat
/cat
/autospeak
/humor
```

В group добавляются:

```text
/yard
/event
/fight
/week
```

В group menu не выносим `/reset`, `/autospeak` и `/humor`. Имя меняется прямо во Дворе одной командой `/name@имя_бота НовоеИмя`.

---

# 7. Что делает `/start`

## Если кота нет

В private chat:

> 🐱 Выбери кота

и четыре кнопки пород.

В группе `/start` не рисует эту карточку, а показывает нейтральную deep-link кнопку.

---

## Если кот уже есть

`/start` фактически становится быстрым Home:

> 🐆 Барсик
> Lv.7
> 😼 Задира
> ⚡ 83/100
>
> [🐾 Тренироваться]
> [👤 Профиль]

В группе `/profile` показывает только public read-only карточку без кнопок настроек.

---

# 8. Создание кота и настройки

Это строго private-only flow. В group/supergroup ни участники, ни админы не должны видеть выбор породы, имя,
подтверждение удаления, humor и autospeak. Все personal callbacks по-прежнему содержат владельца, а в группе
дополнительно отклоняются даже для владельца.

Пример callback:

```text
catui:182:starter:bengal
```

где `182` — внутренний user/session identifier.

При нажатии:

```text
callback_user != owner
    ↓
answerCallbackQuery:
"Это меню Барсика. Открой /start для своего кота."
```

Никакого сообщения в группу.

---

# 9. Shared callback и Personal callback — разные сущности

Это важно заложить архитектурно.

## Personal callback

Например:

* выбрать породу;
* удалить кота;
* переименовать;
* train;
* autospeak.

Имеет:

```text
owner_user_id
```

Нажать может только владелец.

---

## Shared callback

Например:

* выбор действия Yard Event.

Такой callback специально предназначен для любого участника Двора.

При нажатии действие записывается для кота того пользователя, который нажал.

---

# 10. Не редактировать общий shared screen персональными данными

Если Барсик нажал кнопку общего события, нельзя превратить общее сообщение:

> Событие Двора

в:

> Профиль Барсика.

Shared message остаётся shared.

Персональный результат можно:

* показать callback notification;
* отправить отдельным reply;
* либо отдельной owner-bound карточкой.

---

# 11. Именование кота

Имя вводится только в private flow.

Используем Telegram `ForceReply`.

После выбора породы:

> 🐆 Бенгал выбран.
>
> Как зовут кота?

Telegram автоматически открывает reply input именно на сообщение бота.

Бот получит reply на свой prompt даже с включённым Privacy Mode.

---

# 12. `/name`

Работают два варианта.

### Быстрый

```text
/name Барсик
```

### Красивый

```text
/name
```

Бот:

> ✏️ Напиши новое имя Барсика

через selective ForceReply.

---

# 13. Pending input надо переделать

Сейчас pending action фактически связан с пользователем.

При наличии нескольких personal prompt недостаточно хранить только action на пользователя.

Плохо:

```text
user_id → await_cat_name
```

Хорошо:

```text
PendingInput
-----------
user_id
chat_id
kind
prompt_message_id
expires_at
```

Имя принимается только если:

* совпадает user;
* совпадает chat;
* сообщение является reply нужному prompt;
* pending ещё не истёк.

В group/supergroup pending personal input не создаётся; legacy-запрос очищается без изменения кота.

---

# 14. Удаление кота

Команда:

```text
/reset
```

или позднее более понятный alias:

```text
/deletecat
```

Показывает:

> Удалить Барсика и весь его прогресс?

Кнопки:

```text
[Да, удалить]
[Отмена]
```

Confirm callback строго owner-bound.

---

# 15. При удалении кота надо чистить социальное состояние

Проверить cascade/cleanup для:

* `yard_members`;
* relationships;
* незавершённых event choices;
* personality;
* fight history references.

Исторические события можно сохранить с snapshot имени кота.

Но живых ссылок на удалённый `cat_id` остаться не должно.

---

# 16. `/profile` становится главным нативным экраном

Пример:

> 🐆 **Барсик**
>
> Бенгал • Lv.7
> 😼 Задира
>
> ⭐ 381 / 700 XP
> ⚡ 83 / 100
>
> ❤️ 50
> ⚔️ 41
> 🛡 27
> 💨 31
>
> Драки: 8–6
>
> Главный соперник:
> 🐻 Батон — 5 драк

Кнопки:

```text
[🐾 Тренироваться]
[✏️ Имя]

[🗣 Реплики: ON]
[😼 Юмор: BOLD]

[♻️ Удалить кота]
```

Полная карточка с этими кнопками работает только в private. В group/supergroup `/profile` публикует read-only карточку без кнопок.

---

# 17. Тренировка — первое игровое ядро

Training уже идейно правильный.

Игрок нажимает **один раз**.

Не:

> нажать Train 10 раз.

---

# 18. `/train` должен сразу тренировать

Сейчас private flow ещё может сначала открыть Training screen, а потом потребовать кнопку действия.

Для новой концепции:

```text
/train
```

сразу запускает тренировку.

Одно действие.

Один результат.

---

# 19. Кнопка Training в профиле

Тоже запускает действие сразу.

Не открывает промежуточный экран:

> Вы действительно хотите тренироваться?

Это лишний friction.

---

# 20. Energy

Оставляем идею:

```text
max = 100
regen ≈ 1 / 8 минут
```

Training:

```text
минимум = 25
```

и расходует почти всю накопленную энергию.

Текущая хорошая идея:

```text
cost = clamp(energy - 10, 25, 90)
```

---

# 21. Почему Training расходует почти всё

Пользователь не должен заниматься energy optimization:

> мне сделать четыре тренировки по 20 или три по 27?

Он видит:

> Барсик отдохнул.

Нажимает:

> Training.

Готово.

---

# 22. Training — главный источник XP

Training отвечает на простой вопрос:

> Как растёт мой кот?

Ответ:

> Тренируется.

Никаких дополнительных режимов прокачки сейчас не нужно.

---

# 23. Training narrative

Большинство тренировок не требуют LLM.

Используем procedural narrative.

Например:

> 🐆 Барсик тренировался 84 энергии.
>
> Сначала гонял голубя.
> Потом голубь гонял Барсика.
>
> ⭐ +147 XP

---

# 24. AI используется в Training только при интересном контексте

Например:

* level up;
* недавно проиграл rival;
* редкое событие;
* необычно большая тренировка;
* milestone.

После третьего поражения Батону:

> 🐆 Барсик сегодня тренировался подозрительно усердно. Связь с Батоном официально отрицает.

Это делает AI редким и значимым.

---

# 25. Второе игровое ядро — Yard Events

Это основная коллективная механика.

Двор время от времени получает событие.

Например:

> 🐟 У магазина перевернулась машина с рыбой.

Коты выбирают действия.

---

# 26. Не нужен отдельный Expedition

На данном этапе:

**Yard Events уже выполняют функцию коллективного PvE/adventure.**

Поэтому убираем из активного продукта:

```text
/expedition
```

Не удаляем код.

Просто:

* убираем команду из menu;
* убираем кнопку;
* feature flag = off.

---

# 27. Текущий Fish Truck остаётся первым Event

Он уже технически существует.

В C++ уже есть:

```text
ResolveYardEvent
```

с тремя выборами:

```text
STEAL
DISTRACT
SCOUT
```

Это отличный первый prototype.

---

# 28. Событие не должно быть просто фармом

Цель:

> узнать, что сделали другие коты.

Не:

> заработать 13 монет.

Основная награда первого MVP:

* история;
* небольшой XP;
* relationship changes;
* победа/провал Двора;
* личный вклад;
* иногда title/achievement.

---

# 29. Пока убрать loot economy из Event

На MVP не нужны:

* сундуки;
* rarity;
* fragments;
* item upgrades.

Они отвлекают от проверки social loop.

Всё это уже может оставаться в коде экспериментально.

Но пользователь этого пока не видит.

---

# 30. Частота Yard Events

Не несколько раз в день.

Начальная цель:

**примерно одно событие на 24–48 часов активного Двора.**

Это должно быть:

> о, новое событие.

А не:

> ещё одна daily задача.

---

# 31. Время на выбор

Например:

**2–6 часов.**

Человек не обязан присутствовать online.

Увидел сообщение через час → проголосовал.

---

# 32. Event может закончиться раньше

Если:

* участвовало достаточное количество активных котов;
* прошёл минимальный период;

можно разрешить событие раньше.

Но это optimisation later.

На старте достаточно timer.

---

# 33. Выборы лучше сначала скрывать

После нажатия:

> ✅ Барсик сделал выбор.

Но:

> Барсик выбрал «Украсть»

остальным сразу не показываем.

Почему:

возникает surprise при финальном результате.

---

# 34. Иногда нужны публичные cooperative Events

Позднее часть событий может показывать choices.

Например:

> Нужно решить, кто отвлекает собаку.

Но это отдельный event type.

Не усложнять первый.

---

# 35. Первые три события

Не десять.

### Event 1 — Рыба

```text
Украсть
Отвлечь
Разведать
```

### Event 2 — Огромный пёс

```text
Драться
Отвлечь
Обойти
```

### Event 3 — Большая коробка

Больше комедийный event:

```text
Залезть
Охранять
Исследовать
```

Этого достаточно для проверки повторяемости.

---

# 36. Event должен использовать stats

Иначе Training не связан с социальной игрой.

Например:

### Украсть

```text
ATK + SPD
```

### Отвлечь

```text
HP + DEF
```

### Разведать

```text
SPD
```

---

# 37. Разные породы автоматически получают социальные роли

Bengal:

часто хорош в рискованных действиях.

Siamese:

разведка.

British:

отвлечение/защита.

Maine Coon:

выдерживание опасности.

Но никаких hard lock.

---

# 38. Trait пока не влияет на математику

Trait влияет на:

* AI;
* narrative;
* реакции.

Так проще балансировать.

---

# 39. Event result

C++ определяет факты:

```text
success
team_score
participants
MVP
relationship effects
```

AI получает готовый результат.

---

# 40. AI не придумывает outcome

Никогда:

> LLM решил, что Барсик нашёл предмет.

Сначала C++/backend:

```text
Baton failed
Barsik MVP
Smetana scout success
```

Потом AI рассказывает историю.

---

# 41. Третье игровое ядро — драки

Арена локальна для конкретного Двора. Без глобального leaderboard и без поиска по всему Telegram.

---

# 42. Главный UX драки

`/fight` — toggle входа на арену:

1. если кот уже ждёт, команда убирает его из очереди;
2. если ждёт другой кот этого Двора, они сразу матчатся и дерутся;
3. если никого нет, кот встаёт в FIFO-очередь на 15 минут.

Матч атомарен в PostgreSQL и сериализуется по `yard_id`, чтобы два одновременных запроса не остались
двумя ждущими котами.

---

# 43. Выбор соперника

Соперника не нужно выбирать через reply или `@username`: бот сам берёт самого раннего ждущего кота в этом Дворе.

---

# 44. Что происходит после матча

Как только второй кот выходит на арену:

> ⚔️ **Барсик полез на Батона**
>
> 🐆 Барсик Lv.8
> 🐻 Батон Lv.9

C++ считает бой.

Через мгновение:

> 🏆 Батон победил
>
> Осталось HP: 7
> Раундов: 6
>
> Барсик 2–4 Батон

Кнопка:

```text
[😾 Реванш]
```

---

# 45. Реванш

Нажать кнопку может только владелец проигравшего кота.

Это ещё один owner-bound callback.

Получается естественный social loop:

```text
challenge
↓
fight
↓
подкол
↓
revenge
```

---

# 46. Не давать XP за драки

Иначе люди начнут:

> фармить друг друга.

За fight:

* win/loss;
* rivalry;
* streak;
* social history.

Максимум позднее:

* titles;
* небольшие achievements.

---

# 47. Ограничение драки

Не через energy.

Например первоначально:

```text
3 initiated fights / cat / day
```

и:

```text
одна пара не может бесконечно драться подряд
```

Можно разрешить:

* challenge;
* revenge;
* ещё один deciding fight;

после чего небольшой cooldown.

---

# 48. Зачем ограничение

Не ради retention.

Ради чата.

Без ограничения два человека смогут заспамить:

> Барсик vs Батон

40 раз.

---

# 49. Баланс драк

Это надо исправить до выпуска.

Текущий stat growth пород исторически расходился слишком сильно.

Для social fight требуется:

> высокий уровень помогает, но не делает результат абсолютно гарантированным.

Рабочая цель:

### равные уровни

примерно:

```text
45–55%
```

между сбалансированными archetypes.

### +3 уровня

примерно:

```text
60–70%
```

для более сильного.

### +5 уровней

не желательно превращать в:

```text
99.9%
```

---

# 50. Нужен Fight balance simulator

Прогон:

```text
breed A
× breed B
× level difference
× seeds
```

Смотреть:

* win rate;
* rounds;
* remaining HP;
* first-turn advantage;
* crit impact.

---

# 51. Fight RPC

Добавить отдельный C++ contract:

```text
rpc Fight(FightRequest) returns (FightResponse);
```

Не использовать старую Expedition как костыль.

---

# 52. FightRequest

Пример:

```text
rules_version
content_version
seed
attacker
defender
```

---

# 53. FightResponse

```text
winner_cat_id
rounds
final_hp_a
final_hp_b
turn_log
```

---

# 54. Fight state хранится отдельно

Например:

```text
yard_fights
-----------
id
yard_id
attacker_cat_id
defender_cat_id
seed
winner_cat_id
rounds
created_at
```

---

# 55. Relationships

Для MVP достаточно:

```text
friendship
rivalry
```

`respect` пока можно даже не использовать.

---

# 56. Fight меняет rivalry

Например:

```text
обычная драка       +1
реванш              +1
третья подряд       +2
close fight         +1
```

Не надо делать сложную психологическую модель.

---

# 57. Yard Event может менять friendship

Если два кота оказались полезны одной стратегии:

```text
friendship +1
```

Если конкурировали:

```text
rivalry +1
```

---

# 58. AI использует relationships

Например:

```text
Barsik vs Baton
rivalry=8
Barsik lost 3 consecutive fights
```

AI:

> 🐆 Барсик: это не серия поражений. Это затянувшийся сбор аналитики.

---

# 59. AI — четвёртое ядро

Не отдельная игра.

Он делает три игровых механики живыми:

```text
Training
Event
Fight
```

---

# 60. Четыре типа AI-речи

## A. Requested reply

Пользователь сознательно:

```text
/cat
```

reply на сообщение.

---

## B. Ask Cat

```text
/askcat ...
```

---

## C. Event reaction

После важного игрового события кот говорит сам.

---

## D. Cat-to-cat banter

Иногда два кота говорят друг с другом сами.

Это потенциально самая интересная часть.

---

# 61. Requested `/cat`

Уже существующая механика правильная.

Reply на сообщение:

```text
/cat
```

Результат:

> 🐆 Барсик: ...

Никакого impersonation хозяина.

---

# 62. Когда кот говорит сам

Автономная речь не должна зависеть от постоянного чтения всей группы.

На MVP сохраняем Telegram Privacy Mode.

Кот сам говорит на основании:

* Fight;
* Yard Event;
* level-up;
* rivalry milestone;
* ответа человека на сообщение кота;
* редкого scheduler banter.

---

# 63. Почему это важно

Бот **не должен читать весь чат ради возможности иногда пошутить**.

Это:

* дешевле;
* безопаснее;
* меньше privacy concerns;
* меньше prompt injection;
* проще moderation.

Telegram Privacy Mode можно оставить включённым.

---

# 64. Ответ человеку, который reply'нул коту

Очень интересная механика второго этапа.

Если бот ранее отправил:

> 🐆 Барсик: Батон сегодня подозрительно уверен.

Человек reply:

> потому что он тебя опять вынес

Это сообщение бот получает как reply на собственное сообщение.

Можно дать Барсику шанс ответить:

> 🐆 Барсик: я смотрю, свидетелей тоже стало слишком много.

---

# 65. Для этого надо помнить speaker message

Короткая таблица/cache:

```text
cat_message_refs
----------------
telegram_chat_id
telegram_message_id
cat_id
expires_at
```

TTL:

например 24–48 часов.

---

# 66. Cat-to-cat banter

Это не должно быть четыре отдельных сообщения.

Одно компактное сообщение:

> 🐆 Барсик: Батон, одна победа ещё ничего не значит.
>
> 🐻 Батон: согласен. Поэтому у меня их четыре.

Один AI generation может сразу вернуть две реплики.

---

# 67. Когда разрешать banter

Только если:

```text
yard.auto_messages = on
yard.banter = on
owner A autospeak = on
owner B autospeak = on
quiet = off
daily slot available
```

---

# 68. Частота banter

Максимум:

**1 cat-to-cat conversation / Yard / day**

на старте.

И дополнительно максимум:

**1 single-cat autonomous reaction / Yard / day.**

То есть абсолютный максимум:

примерно два автономных AI-moments в сутки.

---

# 69. Idle banter

Иногда коты могут заговорить вообще без свежей драки.

Но context всё равно игровой.

Например:

```text
Барсик:
rival Батон

Батон:
последний event MVP

last fight:
Батон победил
```

Получаем:

> 🐆 Барсик: Батон уже второй день ходит с видом MVP.
>
> 🐻 Батон: а ты уже второй день ходишь без MVP.

---

# 70. Не читать обычную переписку для idle banter

На MVP:

**нет.**

Такое можно рассматривать намного позже как отдельный opt-in experiment.

---

# 71. Первый AI message после создания

Оставляем.

Это хороший момент personality.

После создания:

> 🐻 Батон: хорошо. Имя устраивает. Хозяина пока оценивать рано.

---

# 72. Traits остаются personality

На первом релизе:

* Lazy;
* Bully;
* Philosopher;
* Neat;
* Sleepy.

Они меняют AI voice.

Не combat stats.

---

# 73. Personal humor

Владелец может выбрать:

```text
/humor normal
/humor bold
```

Это preference его кота.

---

# 74. Yard humor

Администратор задаёт maximum allowed tone Двора.

Например:

```text
yard = normal
cat = bold
```

В группе кот говорит:

```text
normal
```

В private:

```text
bold
```

То есть Yard policy всегда ограничивает personal preference.

---

# 75. Настройки Двора — только админам

Текущую идею сохраняем:

```text
/yardsettings
/quiet
```

Но UX исправляем.

---

# 76. Сейчас есть маленькая ошибка

`/yardsettings status` сейчас способен показать настройки **до проверки administrator**.

Это надо переставить.

Правильно:

```text
requireAdmin()
↓
load settings
↓
show/change
```

Для любых вариантов команды.

---

# 77. Админские команды вообще не показывать обычным участникам

Telegram позволяет разные command scopes.

Регистрируем:

### Private

```text
personalCommands
```

### Все пользователи group

```text
personalCommands
+
yardCommands
```

### Group admins

```text
personalCommands
+
yardCommands
+
adminCommands
```

---

# 78. Почему admin list должен быть полным

Admin-specific scope может иметь свой command menu.

Поэтому туда кладём не только:

```text
/yardsettings
/quiet
```

а полный обычный набор + admin additions.

---

# 79. Admin commands

На MVP:

```text
/yardsettings
/quiet
```

Этого достаточно.

---

# 80. Yard Settings

```text
auto messages    on/off
banter           on/off
max auto/day     0/1/2
humor            normal/bold
fights           on/off
```

Последний `fights` можно добавить, чтобы админ мог полностью отключить игровое PvP в неподходящей группе.

---

# 81. Quiet

```text
/quiet 24h
/quiet off
```

отключает:

* autonomous AI;
* idle banter.

Но не отключает:

* `/cat`;
* `/askcat`;
* `/train`;
* `/fight`;
* Event actions.

То есть users могут продолжать играть.

---

# 82. Двор создаётся практически автоматически

Не надо делать `/yard` обязательным setup ritual.

Когда бот находится в group и пользователь впервые выполняет игровую команду:

```text
EnsureYard(chat_id)
EnsureMembership(user, cat)
```

Если yard отсутствует:

создать с безопасными defaults.

---

# 83. `/yard`

Теперь это не:

> «создать Двор».

А:

> **показать состояние Двора.**

Пример:

> 🏘 **Двор «Рабочий чат»**
>
> Котов: 7
>
> Сейчас:
> 🐟 Событие «Рыбовоз»
>
> Выбор сделали: 4/7
>
> Последняя драка:
> Барсик победил Батона
>
> Главные соперники:
> Барсик ↔ Батон

---

# 84. `/event`

Если есть активный Event:

показывает его.

Если пользователь уже сделал выбор:

> ✅ Барсик уже участвует.

Если нет:

кнопки выбора.

---

# 85. Кто запускает Event

Не пользователь каждый раз.

Production-вариант:

**scheduler.**

Он смотрит:

* Yard активен;
* > =2 котов;
* предыдущего Event давно не было.

И создаёт новый.

---

# 86. Для разработки

Можно оставить admin-only:

```text
/eventstart
```

или dev command.

Но не показывать пользователям как gameplay feature.

---

# 87. `/week`

Уже сделан и может остаться.

Но я бы пока **не показывал его в основном command menu**.

Это secondary feature.

Позже:

> итоги недели.

---

# 88. Что убрать из пользовательского command menu сейчас

```text
/expedition
/collection
/bestiary
/bind
/unbind
```

И убрать соответствующие Main Menu buttons.

---

# 89. Код этих features пока не удаляем

Причина:

* он уже написан;
* часть combat/loot кода можно использовать позднее;
* удаление создаст много ненужной работы.

Просто:

```text
feature disabled
```

---

# 90. Новый Main Menu

Вместо нынешнего:

```text
Профиль
Охота
Экспедиция
Арена
Бестиарий
Reset
```

делаем:

```text
🐆 Барсик

[🐾 Тренироваться]
[👤 Профиль]

[🏘 Двор]
[⚔️ Подраться]

[🗣 Спросить кота]
```

Reset находится внутри Profile.

---

# 91. `/fight` не требует выбора соперника

Кнопка «⚔️ Подраться» и команда `/fight` вызывают один и тот же arena toggle. Очередь сама находит пару.

---

# 92. Event buttons остаются shared

Пример:

> 🐟 Машина с рыбой
>
> [😼 Украсть]
> [🐻 Отвлечь]
> [🔎 Разведать]

Каждый нажимает ту же кнопку.

Backend знает пользователя по callback sender.

---

# 93. Визуал без Mini App

Картинки всё равно полезны.

Используем:

* breed avatar;
* level avatar tiers позднее;
* event images;
* fight result cards позднее.

Telegram сам становится visual surface.

---

# 94. Не отправлять картинку на каждое действие

Training:

обычно текст.

Profile:

картинка кота.

Level milestone:

картинка.

Yard Event start:

картинка события.

Fight notable:

можно картинку.

Так изображение остаётся событием, а не spam.

---

# 95. Почему этих трёх механик достаточно для MVP

Потому что они образуют законченный цикл.

```text
TRAIN
↓
кот становится сильнее
↓
EVENT / FIGHT
↓
кот создаёт историю и отношения
↓
AI комментирует произошедшее
↓
друзья реагируют
↓
появляется rivalry / friendship
↓
следующий FIGHT / EVENT уже имеет контекст
```

Это уже игра.

---

# 96. Зачем возвращаться завтра

Не только:

> накопилась энергия.

Есть несколько причин:

### Personal

Барсик снова может тренироваться.

### Social

Произошло новое событие Двора.

### Rivalry

Батон вчера победил Барсика.

### Personality

Коты могут снова устроить короткий banter.

Это значительно сильнее одного progression timer.

---

# 97. Что произойдёт через неделю

В хорошем случае группа уже знает:

> Барсик — агрессивный.

> Батон постоянно его побеждает.

> Сметана почти всегда выбирает Scout.

> Борис регулярно делает какую-то хрень.

Именно это является retention.

---

# 98. Нам пока НЕ нужен Equipment для создания различий

Сначала различия дают:

* breed;
* level;
* trait;
* fight history;
* relationships;
* человеческие решения в Events.

Этого достаточно для теста.

---

# 99. Что добавить первым, если через 2–4 недели станет мало depth

Не inventory из 100 вещей.

Сначала:

### Level 5 perk

один выбор из двух.

Например Bengal:

```text
Засада
или
Хищник
```

### Level 10 perk

ещё один выбор.

Это создаёт builds значительно дешевле Equipment.

---

# 100. Equipment только если реально понадобится

Если пользователи начинают спрашивать:

> как мне сделать Барсика другим?

Тогда рассматриваем 2–3 equipment slots.

Но не раньше.

---

# 101. AI cost model

На одного активного человека:

Requested:

```text
/cat
/askcat
```

только по запросу.

Автономные:

максимум несколько **на весь Yard**, а не на каждого человека.

Event:

одна summary generation.

Fight:

LLM только для notable fights.

Получается контролируемая стоимость.

---

# 102. Какие Fight считаются AI-worthy

Не каждый.

Например:

* 3-я победа подряд над одним rival;
* upset;
* бой закончился на 1–5 HP;
* первый fight пары;
* revenge;
* rivalry milestone.

Обычный fight:

procedural text.

---

# 103. Какие Event автоматически вызывают cat-to-cat banter

Например:

* два кота были MVP/anti-MVP;
* один помог другому;
* rival cats выбрали противоположные роли;
* очень редкий funny outcome.

---

# 104. Автономный разговор не должен быть длинным

Максимум:

**2–4 коротких строки.**

Не:

> Барсик и Батон ведут 20 сообщений AI-dialogue.

Такой бот моментально станет раздражать.

---

# 105. Telegram Privacy Mode оставляем включённым

На MVP бот получает:

* commands;
* callbacks;
* replies боту;
* explicit interactions;
* game events.

Он не анализирует всю человеческую переписку.

Это хороший privacy/product choice.

---

# 106. Если когда-нибудь захотим реакцию на обычный чат

Это отдельная feature:

**Contextual Cats.**

Только:

* admin opt-in;
* прозрачное описание;
* limited rolling context;
* TTL;
* rate limit.

Но сейчас её вообще нет в roadmap.

---

# 107. Router refactor

Сейчас:

```text
onPublicCommand()
onPrivateCommand()
```

имеют сильно разные функции.

Это источник постоянного расхождения.

Переделать на:

```text
handlePersonalCommand()
handleYardCommand()
handleAdminCommand()
```

---

# 108. Новый routing

```text
message
↓
parse command

personal command?
    ↓
handlePersonalCommand

if group:
    ensure yard context

yard command?
    ↓
handleYardCommand

admin command?
    ↓
require admin
    ↓
handleAdminCommand
```

Не должно существовать двух копий logic:

> одна для private, другая для group.

---

# 109. Personal handler

Одинаковый код для:

```text
/start
/profile
/train
/name
/reset
/askcat
/cat
/autospeak
/humor
```

Context также определяет privacy: создание и settings в group не исполняются, а заменяются нейтральной private-chat deep link.

---

# 110. Group handler

Только:

```text
/yard
/event
/fight
/week
```

---

# 111. Admin handler

Только:

```text
/yardsettings
/quiet
```

и внутренние dev-команды при необходимости.

---

# 112. Callback router тоже перестраиваем

Сейчас есть blanket:

```text
if chatType != private:
    return
```

Он больше не подходит.

Вместо него:

```text
callback kind
├── personal sensitive → require owner + require private chat
├── personal game action → require owner
└── shared   → require yard membership
```

---

# 113. Callback должен всегда получать answerCallbackQuery

Даже когда пользователь не имеет права нажать кнопку.

Например:

> Это меню другого кота.

Это лучше, чем молча игнорировать.

---

# 114. Command scopes

`RegisterCommands()` переделать.

Сейчас есть только:

```text
all_private_chats
all_group_chats
```

И admin-команды находятся прямо в общем group list.

Нужно три регистрации:

```text
all_private_chats
all_group_chats
all_chat_administrators
```

---

# 115. Group user commands

Например:

```text
start
profile
train
name
askcat
cat
yard
event
fight
```

---

# 116. Group admin command menu

То же самое плюс:

```text
yardsettings
quiet
```

---

# 117. Admin validation всё равно остаётся backend-side

Command scope — только UX.

Нельзя доверять:

> команда скрыта, значит её никто не вызовет.

Любой пользователь может вручную написать:

```text
/yardsettings auto on
```

Поэтому `getChatMember` check остаётся обязательным.

---

# 118. Исправить `/yardsettings status`

Даже чтение settings:

```text
/yardsettings
/yardsettings status
```

должно сначала пройти:

```text
requireYardAdmin()
```

---

# 119. AI settings model

## Owner

```text
autospeak on/off
personal humor preference
```

## Yard admin

```text
autonomous AI on/off
banter on/off
max 0..2/day
yard humor ceiling
fights on/off
quiet
```

Две независимые permissions.

---

# 120. Чтобы кот заговорил самостоятельно

Нужно:

```text
owner.autospeak == true
AND
yard.auto_messages == true
AND
yard.quiet == false
AND
daily_budget_available
```

Для двух котов:

```text
оба owner.autospeak == true
AND
yard.banter == true
```

---

# 121. Что уже хорошо сделано и сохраняем

Текущая идея hard limit:

```text
max autonomous messages = 2 / Yard / day
```

разумна.

Она предотвращает превращение CatForge в spam bot.

---

# 122. GameEvent layer сохраняем

Все интересные факты оформляются как события:

```text
CatCreated
CatRenamed
CatTrained
CatLeveledUp

YardEventStarted
YardChoiceSubmitted
YardEventResolved

FightStarted
FightFinished
FightRevenge
RivalryMilestone

AutonomousCatMessage
CatBanter
```

---

# 123. AI подписывается не на всё

Например:

```text
CatTrained
```

обычно не AI-worthy.

А:

```text
CatLeveledUp
```

может быть.

```text
FightFinished
```

если обычный — procedural.

```text
ThirdConsecutiveLossToSameCat
```

AI-worthy.

---

# 124. Что делать с текущим Item/Expedition кодом

Заморозить.

Не удалять.

Папки и тесты остаются.

Но public surface их не показывает.

Можно пометить:

```text
FEATURE_LEGACY_EXPEDITION=false
FEATURE_ITEMS=false
```

---

# 125. README надо переписать

Сейчас описание проекта всё ещё будет создавать впечатление:

> PvE expedition + equipment RPG.

Новое README должно описывать:

```text
Training
Yard Events
Cat Fights
AI Personality
Telegram-native social gameplay
```

---

# 126. PLAN.md заменить текущим документом

Старый roadmap содержит много:

* Equipment;
* Expedition;
* Mini App;
* Tournament;
* seasons.

Сейчас это преждевременные ветки.

Оставить их в отдельном:

```text
docs/future_ideas.md
```

Но не в основном PLAN.

---

# 127. Новые метрики MVP

Не:

> сколько coins заработано.

А:

### Training

```text
train_users/day
train_return_rate
```

### Events

```text
event_seen
event_choice_rate
event_repeat_participation
```

### Fights

```text
fight_started
fight_revenge_rate
unique_fight_pairs
```

### AI

```text
requested_cat_reply
autonomous_message
banter_generated
human_reply_to_cat
reaction_to_cat
autospeak_disabled
```

---

# 128. Очень важная метрика — revenge rate

После Fight:

> сколько проигравших нажали «Реванш»?

Если много:

драка эмоционально работает.

---

# 129. Очень важная метрика — event discussion

После Event result:

> появились ли человеческие сообщения/reactions?

Если да:

Event производит social content.

---

# 130. Очень важная AI metric

После автономной реплики:

> человек ответил коту?

Если люди начинают разговаривать **с персонажем**:

Personality layer работает.

---

# 131. Anti-metric

Если растёт:

```text
/autospeak off
/quiet
bot removed
```

AI стал раздражающим.

---

# 132. Phase 0 — убрать лишнее из UI

Первым делом:

1. скрыть Expedition;
2. скрыть Collection;
3. скрыть Bestiary;
4. скрыть bind/unbind;
5. заменить Main Menu;
6. обновить command menus;
7. обновить README/PLAN.

Не удалять underlying код.

---

# 133. Phase 1 — приватное управление и публичная игра

Сделать:

1. создание и settings только в private;
2. `/start` в группе даёт нейтральную deep link;
3. owner-bound callbacks;
4. `/profile` в группе только read-only;
5. `/name` + ForceReply;
6. `/reset` confirm;
7. `/train` direct action;
8. pending input scoped by chat/prompt.

Это сначала.

---

# 134. Definition of Done Phase 1

В private chat пользователь может:

```text
/start
↓
выбрать породу
↓
дать имя
↓
/profile
↓
/train
↓
/name ...
↓
/reset
```

В supergroup ни один из этих personal screens не публикуется; видны только public profile и игровые результаты.

---

# 135. Phase 2 — Fight

Добавить:

1. C++ `Fight`;
2. gRPC contract;
3. FightService;
4. fight persistence;
5. `/fight` arena toggle и FIFO matchmaking;
6. revenge;
7. fight cooldown;
8. win/loss;
9. rivalry;
10. procedural fight story.

---

# 136. Definition of Done Phase 2

В группе первый `/fight` ставит кота в ожидание. Второй `/fight` от другого участника сразу запускает бой.
Повторный `/fight` от ждущего убирает его из очереди.

Получаем результат.

Проигравший может нажать:

> Реванш.

---

# 137. Phase 3 — Yard Events polish

Уже существующий Fish Truck довести до product quality.

Добавить:

1. auto Yard ensure;
2. proper scheduler;
3. event duration;
4. hidden choices;
5. better result;
6. relationship changes;
7. 2 дополнительных Event templates.

---

# 138. Definition of Done Phase 3

Двор может прожить неделю, увидев:

* несколько тренировок;
* несколько драк;
* 3 разных групповых события.

И это уже не выглядит как повтор одной и той же кнопки.

---

# 139. Phase 4 — AI life

После работающих game facts добавить:

1. notable fight reaction;
2. reply-to-cat reaction;
3. real cat-to-cat banter;
4. relationship context;
5. strict daily limits;
6. AI fallback;
7. admin controls.

---

# 140. Definition of Done Phase 4

В группе иногда происходит такое без прямого запроса:

> 🐆 Барсик: Батон слишком много говорит после одной победы.
>
> 🐻 Батон: четырёх.

И участники группы воспринимают это как:

> «эти два опять начали»

а не:

> «бот сгенерировал текст».

---

# 141. Phase 5 — реальный тест

Не писать дальше features.

Взять:

**5–10 настоящих Telegram-групп.**

Пусть используют хотя бы неделю.

Смотреть:

* Training;
* Event participation;
* fights;
* revenge;
* replies котам;
* banter reactions;
* disable rate.

---

# 142. Kill criteria

Если через неделю:

* никто не дерётся;
* события не выбирают;
* котам не отвечают;
* AI выключают;
* люди не помнят имена чужих котов;

дальше не строим RPG.

Меняем core.

---

# 143. Success criteria

Хорошие qualitative признаки:

> «Батон опять охуел.»

> «Барсик ему сейчас реванш даст.»

> «А когда следующее событие?»

> «Добавь этого бота в наш второй чат.»

> «Почему мой кот такой мудак?»

Последняя реплика на самом деле прекрасный product signal.

---

# 144. Что делать после успешного теста

Только тогда решать, чего не хватает.

Возможные направления:

### Недостаточно развития

Добавить Level 5/10 perks.

### Недостаточно разнообразия Events

Добавить новые Event templates.

### Fight быстро надоел

Добавить 1 passive на породу.

### Пользователи хотят внешний вид

Добавить skins/images.

### Пользователи хотят собирать вещи

Вернуть Equipment.

Но решение принимается по поведению игроков.

---

# 145. Что пока НЕ делать

До теста не нужны:

* Mini App;
* Equipment UI;
* новые PvE Expedition;
* bestiary;
* crafting;
* глобальная Arena;
* Tournament;
* Battle Pass;
* seasons;
* marketplace;
* несколько котов;
* realtime PvP;
* сложные skills;
* пять валют.

---

# 146. Новый core loop в одной строке

**Тренирую кота → дерусь/участвую в событии → возникает история → коты это обсуждают → друзья реагируют → отношения меняются → следующая драка/история уже имеет контекст.**

---

# 147. Самое важное отличие новой версии

CatForge больше не строится вокруг:

> «какой content ещё добавить игроку».

Он строится вокруг:

> **«какие новые отношения и истории могут возникнуть между уже существующими котами».**

Это намного дешевле по content production и намного лучше соответствует Telegram.

---

# 148. Финальный MVP

В первом действительно публичном CatForge должны существовать только:

## Personal

* Create Cat;
* Profile;
* Rename;
* Delete;
* Training;
* Ask Cat;
* Reply as Cat;
* AutoSpeak.

## Yard

* Yard status;
* Yard Event;
* Fight;
* Revenge;
* relationships.

## AI

* first personality line;
* requested reply;
* rare autonomous reaction;
* rare cat-to-cat banter.

## Admin

* Yard settings;
* Quiet;
* AI enable;
* banter enable;
* humor;
* fight enable.

И всё.

Если **это** весело в обычном Telegram-чате, у проекта есть основа.

Если нет — никакой Mini App, equipment или красивый boss screen проблему не исправят.
