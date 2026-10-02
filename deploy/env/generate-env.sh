#!/bin/bash
set -euo pipefail

# Директория со скриптом
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEMPLATE_DIR="$SCRIPT_DIR"
COMPOSE_DIR="$SCRIPT_DIR/../compose"

resolve_envsubst() {
  if [ -n "${ENV_SUBST:-}" ] && [ -x "$ENV_SUBST" ]; then
    echo "$ENV_SUBST"
    return 0
  fi
  if [ -n "${ENV_SUBST:-}" ] && command -v "$ENV_SUBST" >/dev/null 2>&1; then
    command -v "$ENV_SUBST"
    return 0
  fi
  if command -v envsubst >/dev/null 2>&1; then
    command -v envsubst
    return 0
  fi
  echo "❌ Ошибка: envsubst не найден (ни ENV_SUBST=$ENV_SUBST, ни в PATH)." >&2
  echo "Установите gettext (envsubst) или положите бинарник в bin/envsubst." >&2
  exit 1
}

ENV_SUBST="$(resolve_envsubst)"

# Загружаем основной .env файл
if [ ! -f "$SCRIPT_DIR/.env" ]; then
  echo "Ошибка: Файл $SCRIPT_DIR/.env не найден!" >&2
  exit 1
fi

# Экспортируем все переменные из .env для использования в envsubst
set -a
# shellcheck disable=SC1091
source "$SCRIPT_DIR/.env"
set +a

# Функция для обработки шаблона и создания .env файла
process_template() {
  local service=$1
  local template="$TEMPLATE_DIR/${service}.env.template"
  local output="$COMPOSE_DIR/${service}/.env"

  echo "Обработка шаблона для сервиса $service..."

  if [ ! -f "$template" ]; then
    echo "⚠️ Шаблон $template не найден, пропускаем..."
    return 0
  fi

  # Создаем директорию, если она еще не существует
  mkdir -p "$(dirname "$output")"

  # Используем envsubst для замены переменных в шаблоне
  "$ENV_SUBST" < "$template" > "$output"

  if [ ! -s "$output" ]; then
    echo "❌ Ошибка: $output пустой после envsubst" >&2
    exit 1
  fi

  echo "✅ Создан файл $output"
}

# Определяем список сервисов из переменной окружения
if [ -z "${SERVICES:-}" ]; then
  echo "⚠️ Переменная SERVICES не задана. Нет сервисов для обработки."
  exit 0
fi

# Разделяем список сервисов по запятой
IFS=',' read -ra services <<< "$SERVICES"
echo "🔍 Обрабатываем сервисы: ${services[*]}"
echo "🛠  envsubst: $ENV_SUBST"

# Обрабатываем шаблоны для всех указанных сервисов
success_count=0
skip_count=0
for service in "${services[@]}"; do
  process_template "$service"
  if [ -f "$TEMPLATE_DIR/${service}.env.template" ]; then
    success_count=$((success_count + 1))
  else
    skip_count=$((skip_count + 1))
  fi
done

if [ "$success_count" -eq 0 ]; then
  echo "⚠️ Ни один .env файл не создан. Проверьте список сервисов и наличие шаблонов."
  exit 1
fi

echo "🎉 Генерация завершена: $success_count файлов создано, $skip_count шаблонов пропущено"
