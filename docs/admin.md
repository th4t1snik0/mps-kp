# Настройка репо (для того, кто его ведёт)

Студентам и их нейронкам это не нужно — им README.md и AGENTS.md.

## Яндекс-диск

Джоба дублирует результаты в папку [MPS-SHEMAS](https://disk.yandex.ru/d/ai0wkudkDT9Tog):
`MPS-SHEMAS/<группа>/<M> <Фамилия И.О.>/`: `СХЕМА-N` (КМ-1: схема PNG/PDF/KiCad, перечни, `params.md`, `vars.inc`, ERC), `ПЗ1-K по СХЕМА-N` (КМ-2), `ПЗ2-K по СХЕМА-N` (КМ-3: ПЗ2 + `программы/` — файлы для робота и отчёт проверки), пробы — `MPS-SHEMAS/_пробы/`; у каждого вида свой счётчик, прошлые не перетираются, недостающие папки создаёт сама.
Нужен OAuth-токен владельца папки в секрете репо `YADISK_TOKEN` (без него шаг пропускается):

1. https://oauth.yandex.ru/client/new → название любое → платформа «Веб-сервисы», Redirect URI
   `https://oauth.yandex.ru/verification_code` → права **Яндекс.Диск REST API**: «Чтение всего Диска»
   и «Запись в любом месте на Диске» → создать, скопировать **ClientID**.
2. Открыть `https://oauth.yandex.ru/authorize?response_type=token&client_id=<ClientID>` под своим аккаунтом,
   разрешить → скопировать токен.
3. GitHub → Settings → Secrets and variables → Actions → **New repository secret**: имя `YADISK_TOKEN`, значение — токен.
4. Другая папка — там же вкладка **Variables** → `YADISK_PUBLIC` = публичная ссылка на папку (папка должна быть опубликована).

Токен живёт ~год и даёт запись во весь Диск — держи на нём только учебное. В чужих форках секрет недоступен.

## Issue «Мой курсач»

- Метка `курсач` должна существовать (Issues → Labels): форма ставит её сама, по ней срабатывает `.github/workflows/issue.yml`.
- Бот коммитит в `main` от `github-actions[bot]` (только `students/<ник>/`) и запускает КМ-1/2/3 через `gh workflow run`
  (пуш токеном GITHUB_TOKEN сам джобы не запускает). Права токену выдают сами workflow (`permissions:` в yml), менять
  настройки репо не нужно — если только организация не запрещает запись токену.
- Команды в Issue принимаются от автора Issue и участников репо (OWNER/MEMBER/COLLABORATOR).
- `/доступ` в Issue — бот упоминает владельца и даёт команду `gh api -X PUT repos/<репо>/collaborators/<логин> -f permission=push`;
  добавляет владелец сам (токен джобы этого не умеет — и не должен).

## Участники

Run workflow в Actions доступен только с правом записи (Settings → Collaborators). Остальные работают через Issue.
