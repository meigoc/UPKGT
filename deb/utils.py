#!/usr/bin/env python3

"""
upkgt-deb - утилиты и вспомогательные функции
Автор: AnmiTaliDev
Дата: 2024-12-30 18:00:55 UTC
"""

import os
import sys
import json
import time
import shutil
import hashlib
import logging
import tarfile
import tempfile
import subprocess
from typing import Dict, List, Tuple, Optional, Union
from datetime import datetime
from pathlib import Path

class UpkgtError(Exception):
    """Базовый класс для ошибок upkgt-deb"""
    pass

# Константы для путей и файлов
LOG_FILE = "/var/log/upkgt-deb.log"
BACKUP_DIR = "/var/backups/upkgt-deb"
CONFIG_DIR = "/etc/upkgt-deb"
CONFIG_FILE = os.path.join(CONFIG_DIR, "config.json")
CACHE_DIR = "/var/cache/upkgt-deb"
LOCK_FILE = "/var/run/upkgt-deb.lock"
DB_PATH = "/var/lib/upkgt-deb/packages.json"

# Требуемые системные пакеты и их зависимости
REQUIRED_PACKAGES = {
    'gpg': {
        'package': 'gnupg',
        'version': '2.2.0',
        'required': True,
        'description': 'GNU Privacy Guard - для проверки подписей'
    },
    'ar': {
        'package': 'binutils',
        'version': '2.30',
        'required': True,
        'description': 'GNU ar - для работы с архивами .deb'
    },
    'tar': {
        'package': 'tar',
        'version': '1.30',
        'required': True,
        'description': 'GNU tar - для работы с архивами tar'
    },
    'xz': {
        'package': 'xz-utils',
        'version': '5.2.0',
        'required': True,
        'description': 'XZ Utils - для работы с архивами xz'
    },
    'zstd': {
        'package': 'zstd',
        'version': '1.4.0',
        'required': False,
        'description': 'Zstandard - для работы с архивами zst'
    }
}

class LockFile:
    """Менеджер контекста для блокировки файла"""
    def __init__(self, lock_path: str):
        self.lock_path = lock_path
        self.locked = False

    def __enter__(self):
        try:
            if os.path.exists(self.lock_path):
                # Проверяем не завис ли предыдущий процесс
                with open(self.lock_path, 'r') as f:
                    pid = int(f.read().strip())
                try:
                    os.kill(pid, 0)
                    raise UpkgtError("Другой процесс уже выполняется")
                except OSError:
                    # Процесс завершился некорректно, удаляем файл блокировки
                    os.unlink(self.lock_path)
            
            # Создаем новый файл блокировки
            with open(self.lock_path, 'w') as f:
                f.write(str(os.getpid()))
            self.locked = True
            return self
        except Exception as e:
            raise UpkgtError(f"Ошибка создания блокировки: {e}")

    def __exit__(self, exc_type, exc_val, exc_tb):
        if self.locked and os.path.exists(self.lock_path):
            try:
                os.unlink(self.lock_path)
            except:
                pass

def setup_logging(debug: bool = False) -> None:
    """
    Настройка системы логирования
    
    Args:
        debug: Включить отладочный режим
    """
    try:
        os.makedirs(os.path.dirname(LOG_FILE), exist_ok=True)
        
        log_format = '%(asctime)s [%(levelname)s] %(message)s'
        level = logging.DEBUG if debug else logging.INFO
        
        handlers = [
            logging.FileHandler(LOG_FILE),
            logging.StreamHandler(sys.stdout)
        ]
        
        logging.basicConfig(
            level=level,
            format=log_format,
            handlers=handlers
        )
        
    except Exception as e:
        print(f"Ошибка настройки логирования: {e}", file=sys.stderr)
        sys.exit(1)

def calculate_file_hash(path: str, algorithm: str = 'sha256') -> str:
    """
    Вычисление хеша файла
    
    Args:
        path: Путь к файлу
        algorithm: Алгоритм хеширования (md5, sha1, sha256)
        
    Returns:
        str: Хеш файла в hex формате
    """
    hash_func = getattr(hashlib, algorithm)()
    
    with open(path, 'rb') as f:
        for chunk in iter(lambda: f.read(4096), b''):
            hash_func.update(chunk)
            
    return hash_func.hexdigest()

def verify_package_integrity(path: str, expected_hash: str) -> bool:
    """
    Проверка целостности пакета
    
    Args:
        path: Путь к файлу пакета
        expected_hash: Ожидаемый хеш
        
    Returns:
        bool: True если хеш совпадает
    """
    actual_hash = calculate_file_hash(path)
    return actual_hash.lower() == expected_hash.lower()

def run_command(cmd: List[str], cwd: Optional[str] = None) -> Tuple[int, str, str]:
    """
    Безопасный запуск команды с таймаутом
    
    Args:
        cmd: Команда и аргументы
        cwd: Рабочая директория
        
    Returns:
        Tuple[int, str, str]: (код возврата, stdout, stderr)
    """
    try:
        process = subprocess.Popen(
            cmd,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            cwd=cwd,
            text=True
        )
        
        stdout, stderr = process.communicate(timeout=60)
        return process.returncode, stdout, stderr
        
    except subprocess.TimeoutExpired:
        process.kill()
        raise UpkgtError("Команда выполнялась слишком долго")
    except Exception as e:
        raise UpkgtError(f"Ошибка выполнения команды: {e}")

def check_root() -> None:
    """Проверка прав суперпользователя"""
    if os.geteuid() != 0:
        raise UpkgtError("Требуются права суперпользователя")

def check_system_requirements() -> List[str]:
    """
    Проверка системных требований
    
    Returns:
        List[str]: Список отсутствующих пакетов
    """
    missing = []
    
    for cmd, info in REQUIRED_PACKAGES.items():
        if not info['required']:
            continue
            
        if not shutil.which(cmd):
            missing.append(info['package'])
            continue
            
        if info.get('version'):
            # Здесь можно добавить проверку версии
            pass
            
    return missing

if __name__ == "__main__":
    print("Этот модуль не предназначен для прямого запуска")
    sys.exit(1)