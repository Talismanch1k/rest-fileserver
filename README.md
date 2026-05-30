# File Server

## Language / Язык

| Language | Quick link |
| --- | --- |
| English | [Go to English section](#english) |
| Русский | [Перейти к русскому разделу](#russian) |

<a id="english"></a>

## English

A small HTTP service for storing files on the local filesystem. The project is written in Go and provides a simple REST API for uploading, reading, updating, deleting, and listing files.

The server is optimized for Linux systems because it uses optimized interaction with system calls to improve performance. It is also protected against path traversal and other vulnerabilities.

### Features

- Upload files through `multipart/form-data`.
- Get a file by name.
- Update an existing file.
- Delete a file.
- Get a sorted list of stored files.
- Limit the maximum incoming file size.
- Configure the service through a YAML file or environment variables.
- Graceful shutdown when the process stops.
- Docker/Compose environment for running the service and integration tests.

### Project Highlights

- **Safe upload directory access.** The service opens the root upload directory through `os.OpenRoot` and performs file operations relative to it, reducing the risk of escaping the storage directory.
- **Filename protection.** The filename is normalized through `filepath.Base`, while service names and empty values are rejected.
- **Temporary files before replacement.** During upload and update, data is first written to `tmp`, then renamed to the target filename.
- **Concurrent access.** Operations for the same filename use lock striping: this protects conflicting operations without blocking the entire storage.
- **Request size limit.** The request body is wrapped in `http.MaxBytesReader`, so oversized files are not written uncontrollably.
- **Minimal infrastructure.** The service does not require a database, message broker, or object storage.

### Project Structure

```text
.
├── compose.yaml              # Docker Compose for the service and test container
├── Makefile                  # commands for running and testing through Docker/Podman
├── fileserver/
│   ├── server.go             # HTTP server startup, configuration, graceful shutdown
│   ├── handlers.go           # HTTP handlers and file storage
│   ├── config.yaml           # configuration example
│   ├── Dockerfile            # production image build for the service
│   └── Makefile              # commands for building/running the service image
└── tests/
    ├── fileserver_test.go    # HTTP API integration tests
    └── Dockerfile            # image for running tests
```

### API

By default, when started through `compose.yaml`, the service is available at `http://localhost:28081`.

#### Upload a File

```bash
curl -i -F "file=@example.txt" http://localhost:28081/files
```

Successful response:

- `201 Created`
- response body contains the created filename

Possible errors:

- `400 Bad Request` - invalid file or filename
- `409 Conflict` - a file with this name already exists
- `413 Request Entity Too Large` - maximum file size exceeded

#### Get a File

```bash
curl -i http://localhost:28081/files/example.txt
```

Successful response:

- `200 OK`
- response body contains the file contents

Possible errors:

- `400 Bad Request` - invalid filename
- `404 Not Found` - file not found

#### Update a File

```bash
curl -i -X PUT -F "file=@new-example.txt" http://localhost:28081/files/example.txt
```

Successful response:

- `200 OK`

Possible errors:

- `400 Bad Request` - invalid file or filename
- `404 Not Found` - the file being updated does not exist
- `413 Request Entity Too Large` - maximum file size exceeded

#### Delete a File

```bash
curl -i -X DELETE http://localhost:28081/files/example.txt
```

Successful response:

- `200 OK`

Deleting a non-existent file also completes successfully.

#### List Files

```bash
curl -i http://localhost:28081/files
```

Successful response:

- `200 OK`
- response body contains filenames, one filename per line
- the list is sorted by name

### Configuration

An example YAML configuration is located at `fileserver/config.yaml`.

Supported parameters:

| YAML field | Environment variable | Default value | Description |
| --- | --- | --- | --- |
| `port` | `FILESERVER_PORT` | required value | HTTP server port |
| `upload_dir` | `FILESERVER_UPLOAD_DIR` | `./uploads` | directory for storing files |
| `max_file_size` | `FILESERVER_MAX_FILE_SIZE` | `10485760` | maximum file size in bytes |

If the YAML file is unavailable, the service tries to read the configuration from environment variables.

### Run with Docker Compose

From the project root:

```bash
make up
```

The command selects `podman` if it is installed, otherwise `docker`, then builds and starts the service through `compose.yaml`.

Stop the environment:

```bash
make down
```

When started through Compose, container port `8080` is published to host port `28081`.

### Run Locally Without a Container

Go `1.25.1` or a compatible version is required.

```bash
cd fileserver
go run . -config config.yaml
```

After startup, the service listens on the port from the configuration. In the current example, it is `1234`:

```bash
curl http://localhost:1234/files
```

You can also run it only with environment variables:

```bash
cd fileserver
FILESERVER_PORT=8080 FILESERVER_UPLOAD_DIR=./uploads go run .
```

### Tests

Integration tests are located in the `tests` module and cover the main HTTP API scenarios:

- creating files;
- listing files;
- reading a file;
- reading a missing file;
- uploading an existing file again;
- updating a file;
- updating a missing file.

Run the full container-based cycle:

```bash
make test
```

This command stops the old environment, builds and starts the service, runs the test container with `go test -race -v ./...`, then stops the environment.

### Main Technologies

- Go `net/http` and routes like `GET /files/{filename}`.
- `cleanenv` for reading configuration from YAML and environment variables.
- Docker/Podman for reproducible startup.
- `testify` for integration tests.

<a id="russian"></a>

## Русский

Небольшой HTTP-сервис для хранения файлов на локальной файловой системе. Проект реализован на Go и предоставляет простой REST API для загрузки, чтения, обновления, удаления и просмотра списка файлов.

Работа сервера оптимизирована и нацелена на Linux-системы, так как он использует оптимизированное взаимодействие с системными вызовами для ускорения работы. Также сервис защищен от path traversal и других уязвимостей.

### Возможности

- Загрузка файлов через `multipart/form-data`.
- Получение файла по имени.
- Обновление существующего файла.
- Удаление файла.
- Получение отсортированного списка сохраненных файлов.
- Ограничение максимального размера входящего файла.
- Конфигурация через YAML-файл или переменные окружения.
- Graceful shutdown при остановке процесса.
- Docker/Compose-окружение для запуска сервиса и интеграционных тестов.

### Фишки проекта

- **Безопасная работа с директорией загрузок.** Сервис открывает корневую директорию через `os.OpenRoot` и выполняет файловые операции относительно нее, что снижает риск выхода за пределы хранилища.
- **Защита имени файла.** Имя нормализуется через `filepath.Base`, а служебные и пустые значения отклоняются.
- **Временные файлы перед заменой.** При загрузке и обновлении данные сначала записываются в `tmp`, после чего файл переименовывается в целевое имя.
- **Конкурентный доступ.** Для операций над одним и тем же именем файла используется lock-striping: это защищает конфликтующие операции, но не блокирует все хранилище целиком.
- **Ограничение размера запроса.** Тело запроса оборачивается в `http.MaxBytesReader`, поэтому слишком большие файлы не записываются бесконтрольно.
- **Минимальная инфраструктура.** Для работы не нужны база данных, брокер сообщений или объектное хранилище.

### Структура проекта

```text
.
├── compose.yaml              # Docker Compose для сервиса и тестового контейнера
├── Makefile                  # команды запуска и тестирования через Docker/Podman
├── fileserver/
│   ├── server.go             # запуск HTTP-сервера, конфигурация, graceful shutdown
│   ├── handlers.go           # HTTP-handlers и файловое хранилище
│   ├── config.yaml           # пример конфигурации
│   ├── Dockerfile            # сборка production-образа сервиса
│   └── Makefile              # команды для сборки/запуска образа сервиса
└── tests/
    ├── fileserver_test.go    # интеграционные тесты HTTP API
    └── Dockerfile            # образ для запуска тестов
```

### API

По умолчанию при запуске через `compose.yaml` сервис доступен на `http://localhost:28081`.

#### Загрузить файл

```bash
curl -i -F "file=@example.txt" http://localhost:28081/files
```

Успешный ответ:

- `201 Created`
- тело ответа содержит имя созданного файла

Возможные ошибки:

- `400 Bad Request` - некорректный файл или имя файла
- `409 Conflict` - файл с таким именем уже существует
- `413 Request Entity Too Large` - превышен максимальный размер файла

#### Получить файл

```bash
curl -i http://localhost:28081/files/example.txt
```

Успешный ответ:

- `200 OK`
- тело ответа содержит содержимое файла

Возможные ошибки:

- `400 Bad Request` - некорректное имя файла
- `404 Not Found` - файл не найден

#### Обновить файл

```bash
curl -i -X PUT -F "file=@new-example.txt" http://localhost:28081/files/example.txt
```

Успешный ответ:

- `200 OK`

Возможные ошибки:

- `400 Bad Request` - некорректный файл или имя файла
- `404 Not Found` - обновляемый файл не существует
- `413 Request Entity Too Large` - превышен максимальный размер файла

#### Удалить файл

```bash
curl -i -X DELETE http://localhost:28081/files/example.txt
```

Успешный ответ:

- `200 OK`

Удаление несуществующего файла также завершается успешно.

#### Получить список файлов

```bash
curl -i http://localhost:28081/files
```

Успешный ответ:

- `200 OK`
- тело ответа содержит имена файлов, по одному имени на строку
- список отсортирован по имени

### Конфигурация

Пример YAML-конфигурации находится в `fileserver/config.yaml`.

Поддерживаемые параметры:

| YAML-поле | Переменная окружения | Значение по умолчанию | Описание |
| --- | --- | --- | --- |
| `port` | `FILESERVER_PORT` | обязательное значение | порт HTTP-сервера |
| `upload_dir` | `FILESERVER_UPLOAD_DIR` | `./uploads` | директория для хранения файлов |
| `max_file_size` | `FILESERVER_MAX_FILE_SIZE` | `10485760` | максимальный размер файла в байтах |

Если YAML-файл недоступен, сервис пытается прочитать конфигурацию из переменных окружения.

### Запуск через Docker Compose

В корне проекта:

```bash
make up
```

Команда выберет `podman`, если он установлен, иначе `docker`, затем соберет и запустит сервис через `compose.yaml`.

Остановить окружение:

```bash
make down
```

При Compose-запуске порт контейнера `8080` пробрасывается на порт хоста `28081`.

### Локальный запуск без контейнера

Требуется Go `1.25.1` или совместимая версия.

```bash
cd fileserver
go run . -config config.yaml
```

После запуска сервис будет слушать порт из конфигурации. В текущем примере это `1234`:

```bash
curl http://localhost:1234/files
```

Также можно запустить только через переменные окружения:

```bash
cd fileserver
FILESERVER_PORT=8080 FILESERVER_UPLOAD_DIR=./uploads go run .
```

### Тесты

Интеграционные тесты находятся в модуле `tests` и проверяют основные сценарии HTTP API:

- создание файлов;
- получение списка;
- чтение файла;
- чтение отсутствующего файла;
- повторную загрузку существующего файла;
- обновление файла;
- обновление отсутствующего файла.

Запуск полного цикла через контейнеры:

```bash
make test
```

Эта команда останавливает старое окружение, собирает и запускает сервис, запускает тестовый контейнер с `go test -race -v ./...`, затем останавливает окружение.

### Основные технологии

- Go `net/http` и маршруты вида `GET /files/{filename}`.
- `cleanenv` для чтения конфигурации из YAML и окружения.
- Docker/Podman для воспроизводимого запуска.
- `testify` для интеграционных тестов.
