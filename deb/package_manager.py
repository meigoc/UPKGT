#!/usr/bin/env python3

"""
upkgt-deb - менеджер пакетов
Автор: AnmiTaliDev
Дата: 2024-12-30 18:14:18 UTC
"""

import os
import json
import shutil
import logging
import tempfile
from typing import Dict, List, Set, Optional
from datetime import datetime
from pathlib import Path

from package import Package, PackageError
from utils import LockFile, check_root, check_system_requirements

class PackageManagerError(Exception):
    """Базовый класс для ошибок менеджера пакетов"""
    pass

class PackageManager:
    """Класс для управления пакетами"""
    
    def __init__(self):
        self.db_path = "/var/lib/upkgt-deb/packages.json"
        self.backup_dir = "/var/backups/upkgt-deb"
        self.cache_dir = "/var/cache/upkgt-deb"
        self.lock_file = "/var/run/upkgt-deb.lock"
        
        # Создаем необходимые директории
        for directory in [
            os.path.dirname(self.db_path),
            self.backup_dir,
            self.cache_dir
        ]:
            os.makedirs(directory, exist_ok=True)
        
        # Загружаем базу данных
        self.packages = self._load_database()
        
        # Множество для отслеживания изменений
        self._modified = False

    def _load_database(self) -> Dict:
        """Загрузка базы данных установленных пакетов"""
        if os.path.exists(self.db_path):
            try:
                with open(self.db_path, 'r') as f:
                    return json.load(f)
            except json.JSONDecodeError:
                logging.error("Поврежденная база данных, создаем новую")
                self._backup_database()
        return {}

    def _save_database(self) -> None:
        """Сохранение базы данных"""
        if not self._modified:
            return
            
        try:
            # Создаем бэкап перед сохранением
            self._backup_database()
            
            # Сохраняем новую версию
            with open(self.db_path, 'w') as f:
                json.dump(self.packages, f, indent=2)
            
            self._modified = False
            logging.info("База данных успешно сохранена")
            
        except Exception as e:
            raise PackageManagerError(f"Ошибка сохранения базы данных: {e}")

    def _backup_database(self) -> None:
        """Создание резервной копии базы данных"""
        if not os.path.exists(self.db_path):
            return
            
        timestamp = datetime.now().strftime('%Y%m%d_%H%M%S')
        backup_path = os.path.join(
            self.backup_dir,
            f"packages.{timestamp}.json"
        )
        
        try:
            shutil.copy2(self.db_path, backup_path)
            logging.info(f"Создана резервная копия: {backup_path}")
            
            # Удаляем старые бэкапы (оставляем последние 5)
            self._cleanup_backups()
            
        except Exception as e:
            logging.error(f"Ошибка создания резервной копии: {e}")

    def _cleanup_backups(self, max_backups: int = 5) -> None:
        """Очистка старых резервных копий"""
        backups = []
        
        for f in os.listdir(self.backup_dir):
            if f.startswith('packages.') and f.endswith('.json'):
                path = os.path.join(self.backup_dir, f)
                backups.append((os.path.getmtime(path), path))
        
        # Сортируем по времени создания (старые в начале)
        backups.sort()
        
        # Удаляем лишние бэкапы
        while len(backups) > max_backups:
            _, path = backups.pop(0)
            try:
                os.unlink(path)
                logging.info(f"Удалена старая резервная копия: {path}")
            except OSError as e:
                logging.warning(f"Ошибка удаления {path}: {e}")

    def install(self, pkg_path: str, force: bool = False) -> None:
        """
        Установка пакета
        
        Args:
            pkg_path: Путь к .deb файлу
            force: Принудительная установка
        """
        check_root()
        missing_deps = check_system_requirements()
        if missing_deps:
            raise PackageManagerError(
                f"Отсутствуют необходимые пакеты: {', '.join(missing_deps)}"
            )
        
        with LockFile(self.lock_file):
            try:
                with Package(pkg_path) as pkg:
                    logging.info(f"Начало установки пакета: {pkg_path}")
                    
                    # Извлекаем информацию о пакете
                    pkg.extract()
                    pkg.extract_control_info()
                    
                    name = pkg.control.get('package')
                    version = pkg.control.get('version')
                    
                    if not name or not version:
                        raise PackageManagerError(
                            "Не удалось определить имя или версию пакета"
                        )
                    
                    # Проверяем конфликты файлов
                    conflicts = set(pkg.get_installed_files())
                    if conflicts and not force:
                        raise PackageManagerError(
                            f"Обнаружены конфликты файлов:\n"
                            f"{chr(10).join(conflicts)}\n"
                            "Используйте --force для принудительной установки"
                        )
                    
                    # Запускаем preinst скрипт
                    pkg.run_maintainer_script('preinst')
                    
                    # Устанавливаем файлы
                    pkg.install_files()
                    
                    # Запускаем postinst скрипт
                    pkg.run_maintainer_script('postinst')
                    
                    # Обновляем базу данных
                    self.packages[name] = {
                        'version': version,
                        'files': pkg.files,
                        'maintainer': pkg.control.get('maintainer', ''),
                        'description': pkg.control.get('description', ''),
                        'depends': pkg.dependencies,
                        'provides': list(pkg.provides),
                        'replaces': list(pkg.replaces),
                        'install_date': datetime.now().isoformat()
                    }
                    
                    self._modified = True
                    self._save_database()
                    
                    logging.info(f"Пакет {name} {version} успешно установлен")
                    
            except Exception as e:
                logging.error(f"Ошибка установки пакета: {e}")
                raise

if __name__ == "__main__":
    print("Этот модуль не предназначен для прямого запуска")
    sys.exit(1)