# Материалы прошлых лет — где лежат

Сырые файлы (ПЗ, схемы, архивы, скриншоты тестов) в git не кладём — тяжело. Локально их принято
держать в `docs/prev/<источник>/` (папка в `.gitignore`). Выжимки из них — в `docs/research/`.

| Источник | Что там | Локально |
| --- | --- | --- |
| [mail.ru — КП по МПС (группа 2025)](https://cloud.mail.ru/public/TeV3/qEfCeiDza/7%20%D0%A1%D0%95%D0%9C%D0%95%D0%A1%D0%A2%D0%A0/%D0%9A%D0%9F%20%D0%BF%D0%BE%20%D0%BC%D0%B8%D0%BA%D1%80%D0%BE%D0%BF%D1%80%D0%BE%D1%86%D0%B5%D1%81%D1%81%D0%BE%D1%80%D0%BD%D1%8B%D0%BC%20%D1%81%D0%B8%D1%81%D1%82%D0%B5%D0%BC%D0%B0%D0%BC) | принятые Э3/ПЭ/ПЗ1/ПЗ2 (Осипова, 2025), защита («ДИСКИ»), тест КМ-4 с ответами, KiCad ГОСТ-библиотека, даташиты | `docs/prev/mailru-group/` |
| [mail.ru — МПС (курс)](https://cloud.mail.ru/public/TeV3/qEfCeiDza/7%20%D0%A1%D0%95%D0%9C%D0%95%D0%A1%D0%A2%D0%A0/%D0%9C%D0%B8%D0%BA%D1%80%D0%BE%D0%BF%D1%80%D0%BE%D1%86%D0%B5%D1%81%D1%81%D0%BE%D1%80%D0%BD%D1%8B%D0%B5%20%D1%81%D0%B8%D1%81%D1%82%D0%B5%D0%BC%D1%8B) | лекции ч1/ч2, лабораторные, тесты 1–2 | `docs/prev/mailru-mps-course/` |
| [MEGA](https://mega.nz/folder/WwM0zBiR#sLnZimqLKkcpvDhRt1UHRg/folder/OlVkyBob) (МЭИ ВМКСС/7 семестр/МПС/Курсовая) | Базарнов, Хромочкина (2023), типовые замечания по схемам, вопросы защиты, даташит IDT7005 (рус./англ.), примеры 2017–2019 | `docs/prev/mega-bazarnov/` |
| [Яндекс-диск](https://disk.yandex.ru/d/e_wJRXdobDVMIg/7th%20term/%D0%9C%D0%9F%D0%A1/%D0%BA%D1%83%D1%80%D1%81%D0%BE%D0%B2%D0%B0%D1%8F%20(%D0%B7%D0%B0%D0%BB%D1%83%D0%BF%D0%B0)) | Женюх (А-07-19), Сорокина (А-12-20): ПЗ, замечания руководителя, readme про защиту | `docs/prev/yandex-zhenyukh-sorokina/` |
| [Яндекс-диск](https://disk.yandex.ru/d/z3XcM_yY86YxEg/%D0%9C%D0%9F%D0%A1/%D0%BA%D1%83%D1%80%D1%81%D0%B0%D1%87) | Козлова (А-12-19): курсовая с расчётами, код, вопросы на защиту | `docs/prev/yandex-kozlova/` |
| [GitHub lordcoudy/MPEI](https://github.com/lordcoudy/MPEI/tree/main/7_%D1%81%D0%B5%D0%BC%D0%B5%D1%81%D1%82%D1%80/%D0%9C%D0%B8%D0%BA%D1%80%D0%BE%D0%BF%D1%80%D0%BE%D1%86%D0%B5%D1%81%D1%81%D0%BE%D1%80%D0%BD%D1%8B%D0%B5_%D0%A1%D0%B8%D1%81%D1%82%D0%B5%D0%BC%D1%8B/%D0%9A%D1%83%D1%80%D1%81%D0%B0%D1%87) | Кретов, Балашов (А-08-19), асм, шрифт GOST A, трафареты Visio | `docs/prev/github-lordcoudy/` |

Как скачивать без браузера:
- Яндекс: `https://cloud-api.yandex.net/v1/disk/public/resources?public_key=<ссылка>&path=<путь>` (листинг),
  `…/resources/download?…` (ссылка на файл).
- mail.ru: листинг `https://cloud.mail.ru/api/v4/public/list?weblink=<путь>`, файлы — через сервер из
  `https://cloud.mail.ru/api/v2/dispatcher` (`weblink_get`). Облако режет частые запросы — паузы 5–10 с.
- MEGA: файлы зашифрованы ключом из ссылки (после `#`); проще скачать в браузере.

**Внимание:** работы 2017–2023 делались по другому ТЗ (АЦП, UART, 2 разряда, внешнее устройство буфер читает).
Что изменилось — `docs/research/remarks-and-defense.md`, раздел D.
