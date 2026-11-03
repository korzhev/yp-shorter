# internal/config

Пакет загружает настройки приложения. Приоритет (от высшего к низшему):

1. Переменные окружения.
2. Явно переданные флаги.
3. JSON-файл.
4. Значения по умолчанию.

Путь к файлу задаётся через `-c`, `-config` или `CONFIG`. Непустой `CONFIG`
имеет приоритет над флагами пути. Без пути файл не загружается.
Отсутствующие JSON-поля сохраняют значения по умолчанию. Ошибки чтения,
синтаксиса JSON и типов значений возвращаются вызывающему коду.
Комментарии в JSON не поддерживаются.

| Поле JSON | Переменная окружения | Флаг |
| --- | --- | --- |
| `server_address` | `SERVER_ADDRESS` | `-a` |
| `base_url` | `BASE_URL` | `-b` |
| `file_storage_path` | `FILE_STORAGE_PATH` | `-f` |
| `database_dsn` | `DATABASE_DSN` | `-d` |
| `enable_https` | `ENABLE_HTTPS` | `-s` |
| `tls_cert_file` | `TLS_CERT_FILE` | `-tls-cert` |
| `tls_key_file` | `TLS_KEY_FILE` | `-tls-key` |
| `sh_charset` | `SH_CHARSET` | `-charset` |
| `sh_length` | `SH_LENGTH` | `-l` |
| `log_level` | `LOG_LEVEL` | `-ll` |
| `token_secret` | `TOKEN_SECRET` | `-ts` |
| `token_exp_minutes` | `TOKEN_EXP_MINUTES` | `-te` |
| `audit_file` | `AUDIT_FILE` | `-audit-file` |
| `audit_url` | `AUDIT_URL` | `-audit-url` |
| `use_profiler` | `USE_PROFILER` | `-p` |

Флаг алфавита переименован из `-c` в `-charset`, поскольку `-c` теперь выбирает
файл конфигурации. Тип хранилища вычисляется после объединения настроек:
непустой DSN → PostgreSQL, иначе непустой путь хранения → файл, иначе память.
