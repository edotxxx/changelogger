# Changelogger

Changelogger - это консольный инструмент, который автоматически генерирует блок для `CHANGELOG.md` по git-коммитам.

Go-версия поставляется как один бинарник и не требует PHP, Composer или Packagist на машине пользователя.

## Требования к окружению

Для работы Changelogger нужны:

- Git;
- репозиторий с веткой `develop` и `origin/master`;
- git tags в формате `X.Y.Z`, например `1.24.5`;
- файл `.env` в каталоге запуска;
- файл `CHANGELOG.md` в корне проекта или путь к нему в `CHANGELOG_PATH`.

## Установка

Для установки или обновления выполните:

```bash
curl -fsSL https://github.com/hiddenpathz/changelogger/releases/latest/download/changelogger-install | sh
```

Installer сам определит ОС и установит бинарник в:

```text
/usr/local/bin/changelogger
```

Если нужен другой каталог установки:

```bash
curl -fsSL https://github.com/hiddenpathz/changelogger/releases/latest/download/changelogger-install | CHANGELOGGER_INSTALL_DIR="$HOME/bin" sh
```

После установки команда доступна из любого проекта:

```bash
changelogger
```

## Подготовка проекта для работы

Добавьте в `.env` проекта:

- ссылку на репозиторий до номера тега;
- префикс ветки, если он нужен по правилам проекта;
- название системы задач, если в changelog нужны ссылки на задачи;
- ссылку на систему задач до кода заявки или шаблон с `{code}`;
- путь до `CHANGELOG.md`, если он находится не рядом с `.env`.

```env
REPOSITORY_LINK=https://gitlab.some.ru/your.repo.ru/-/tags/
BRANCH_PREFIX=MYPROJECT
TASK_SYSTEM_NAME=SomeTaskSystemName
TASK_SYSTEM_LINK=https://some-task-system.ru/tasks/view?code=
CHANGELOG_PATH=./CHANGELOG.md
```

`TASK_SYSTEM_LINK` можно указывать в одном из форматов:

```env
TASK_SYSTEM_LINK=https://some-task-system.ru/tasks/view?code=
TASK_SYSTEM_LINK=https://some-task-system.ru/tasks/view?code={code}
TASK_SYSTEM_LINK=https://some-task-system.ru
```

## Запуск

Запускать нужно из каталога проекта, где лежит `.env`:

```bash
changelogger
```

Можно явно передать ссылку на репозиторий до номера тега. В этом случае аргумент переопределит `REPOSITORY_LINK` из `.env`:

```bash
changelogger https://gitlab.some.ru/your.repo.ru/-/tags/
```

## Использование

- При старте программа предложит выбрать ветку, от которой нужно собрать changelog.
- Если просто нажать Enter, changelog будет собран от `develop`.
- В подсказках показываются свежие ветки `release/*`, `hotfix/*`, `origin/release/*`, `origin/hotfix/*` и текущая ветка, если она не является служебной changelog-веткой.
- Затем Changelogger попросит ввести номер заявки и задачи в формате `IU000000-W0111111`.

Пример выбора ветки:

```text
Из какой ветки собрать changelog?

1 - develop [по умолчанию]
2 - release/1.24.0
3 - release/1.25.0
4 - текущая ветка: feature/ABC-123-something
0 - ввести вручную

Выбор [1]:
```

Ветки вида `feature/*-assign-to-changelog` не предлагаются как источник changelog. Если отменить выполнение на вопросе `Все верно?`, CLI вернется на выбранную ветку-источник. После подтверждения текста changelog отказ от коммита или push оставит пользователя в changelog-ветке для ручных правок.
Если выбрана ветка вида `origin/release/*` или `origin/hotfix/*`, Changelogger сначала создаст для нее локальную tracking-ветку без префикса `origin/`, чтобы не оставлять репозиторий в detached HEAD.

После ввода номера заявки будет создана ветка:

```text
feature/MYPROJECT-IU000000-W0111111-assign-to-changelog
```

Если `BRANCH_PREFIX` не указан:

```text
feature/IU000000-W0111111-assign-to-changelog
```

Далее программа покажет текущую версию и попросит выбрать, какую часть версии поднять:

```bash
Текущая версия приложения: 1.24.5
Какую версию нужно поднять?
 1 - major (*.0.0)
 2 - minor (0.*.0)
 3 - fix   (0.0.*)
```

После выбора Changelogger выведет изменения, которые попадут в `CHANGELOG.md`, и попросит подтверждение.

## Правила коммитов

В changelog попадают коммиты с префиксами:

| Ключ     | Запись      |
|----------|-------------|
| feat     | Реализовано |
| refactor | Изменено    |
| change   | Изменено    |
| fix      | Исправлено  |
| remove   | Удалено     |

Секции всегда выводятся в регламентном порядке:

```text
Реализовано
Изменено
Исправлено
Удалено
```

Описание после префикса поднимается с первой буквы в uppercase:

```bash
[IDPSPS-IU212256-W0835055] fix: исправлены опечатки
```

В `CHANGELOG.md` попадет:

```md
- Исправлены опечатки
```

Префиксы `refactor` и `change` равнозначны и оба попадают в секцию `Изменено`:

```bash
[IDPSPS-IU212256-W0835055] change: обновлен расчет скидок
```

В `CHANGELOG.md` попадет:

```md
- Изменено:
  - Обновлен расчет скидок
```

Коммиты с другими префиксами не попадут в `CHANGELOG.md`, например:

```md
wip - промежуточные коммиты в процессе работы
ci - настройки CI/CD
build - настройки окружения без пользовательского смысла
```

## Пример результата

```md
# История изменений

## [ [1.1.0](https://gitlab.some.ru/your.repo.ru/-/tags/1.1.0) ] - 01.01.2023

- Реализовано:
  - Отображение кода заявки
  - Учет параметра "Срочная доставка"
- Изменено:
  - Дополнены правила расчета стоимости
- Исправлено:
  - Некорректные названия полей в форме
- Удалено:
  - Удален устаревший экран настроек
```

## Завершение работы

После записи `CHANGELOG.md` программа предложит:

- создать commit;
- push ветки;
- удалить локальную ветку после push.

Если на вопрос ответить `n`, действие будет отменено. Изменения можно будет завершить вручную.

## Сборка из исходников

Для локальной сборки нужен Go:

```bash
go build -o ./bin/changelogger ./cmd/changelogger
```

Проверка:

```bash
go test ./...
```

## Публикация релиза без CI

Соберите release-артефакты локально:

```bash
./scripts/build-release.sh
```

Скрипт создаст:

```text
dist/changelogger-darwin-universal
dist/changelogger-linux-amd64
dist/changelogger-install
dist/checksums.txt
```

Создайте GitHub Release в `https://github.com/hiddenpathz/changelogger` и приложите все файлы из `dist/`.

После публикации проверьте установку:

```bash
curl -fsSL https://github.com/hiddenpathz/changelogger/releases/latest/download/changelogger-install | sh
changelogger
```

Локально проверить installer без GitHub Releases можно так:

```bash
./scripts/build-release.sh
CHANGELOGGER_BASE_URL="file://$PWD/dist" CHANGELOGGER_INSTALL_DIR="$HOME/bin" ./changelogger-install
```
