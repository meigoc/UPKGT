#!/usr/bin/env python3

"""
upkgt-deb - модуль для работы с .deb пакетами
Автор: AnmiTaliDev
Дата: 2024-12-30 18:12:13 UTC
"""

import os
import re
import json
import shutil
import logging
import tarfile
import tempfile
import subprocess
from typing import Dict, List, Optional, Set, Tuple
from datetime import datetime
from pathlib import Path

class PackageError(Exception):
    """Базовый класс для ошибок работы с пакетами"""
    pass

class Package:
    """Класс для работы с .deb пакетами"""
    
    def __init__(self, path: str):
        self.path = os.path.abspath(path)
        self.temp_dir: Optional[str] = None
        self.control: Dict = {}
        self.files: List[str] = []
        self.conflicts: Set[str] = set()
        self.dependencies: Dict[str, str] = {}
        self.provides: Set[str] = set()
        self.replaces: Set[str] = set()
        self.maintainer_scripts: Dict[str, str] = {}
        
        # Проверяем существование файла
        if not os.path.isfile(self.path):
            raise PackageError(f"Файл не найден: {self.path}")
            
        # Проверяем расширение файла
        if not self.path.endswith('.deb'):
            raise PackageError("Файл должен иметь расширение .deb")
            
        # Создаем временную директорию
        self.temp_dir = tempfile.mkdtemp(prefix='upkgt-')

    def __enter__(self):
        return self

    def __exit__(self, exc_type, exc_val, exc_tb):
        self.cleanup()

    def cleanup(self) -> None:
        """Очистка временных файлов"""
        if self.temp_dir and os.path.exists(self.temp_dir):
            try:
                shutil.rmtree(self.temp_dir)
                self.temp_dir = None
            except Exception as e:
                logging.warning(f"Ошибка при очистке временных файлов: {e}")

    def extract(self) -> None:
        """Распаковка .deb пакета"""
        try:
            subprocess.run(
                ['ar', 'x', self.path],
                cwd=self.temp_dir,
                check=True,
                capture_output=True
            )
        except subprocess.CalledProcessError as e:
            raise PackageError(f"Ошибка распаковки пакета: {e.stderr.decode()}")

    def get_archive_path(self, prefix: str) -> str:
        """
        Определение пути к архиву в пакете
        
        Args:
            prefix: Префикс архива (control или data)
            
        Returns:
            str: Путь к найденному архиву
        """
        patterns = [
            f"{prefix}.tar.xz",
            f"{prefix}.tar.gz",
            f"{prefix}.tar.zst",
            f"{prefix}.tar"
        ]
        
        for pattern in patterns:
            path = os.path.join(self.temp_dir, pattern)
            if os.path.exists(path):
                return path
                
        raise PackageError(f"Архив {prefix}.tar.* не найден")

    def extract_control_info(self) -> None:
        """Извлечение информации из control файла"""
        control_archive = self.get_archive_path('control')
        temp_dir = tempfile.mkdtemp()
        
        try:
            # Распаковываем control.tar.*
            subprocess.run(
                ['tar', 'xf', control_archive, '-C', temp_dir],
                check=True,
                capture_output=True
            )
            
            # Читаем control файл
            control_file = os.path.join(temp_dir, './control')
            current_key = None
            current_value = []
            
            with open(control_file, 'r', encoding='utf-8') as f:
                for line in f:
                    if line.startswith(' '):
                        # Продолжение предыдущего поля
                        current_value.append(line.strip())
                    elif ':' in line:
                        # Сохраняем предыдущее поле если есть
                        if current_key:
                            self.control[current_key] = '\n'.join(current_value)
                        
                        # Начинаем новое поле
                        key, value = line.split(':', 1)
                        current_key = key.strip().lower()
                        current_value = [value.strip()]
                
                # Сохраняем последнее поле
                if current_key:
                    self.control[current_key] = '\n'.join(current_value)
            
            # Извлекаем зависимости
            if 'depends' in self.control:
                self._parse_dependencies(self.control['depends'])
            
            # Извлекаем provides
            if 'provides' in self.control:
                self.provides = set(
                    p.strip() for p in self.control['provides'].split(',')
                )
            
            # Извлекаем replaces
            if 'replaces' in self.control:
                self.replaces = set(
                    r.strip() for r in self.control['replaces'].split(',')
                )
            
            # Сохраняем maintainer скрипты
            for script in ['preinst', 'postinst', 'prerm', 'postrm']:
                script_path = os.path.join(temp_dir, f"./{script}")
                if os.path.exists(script_path):
                    with open(script_path, 'r') as f:
                        self.maintainer_scripts[script] = f.read()
                        
        finally:
            shutil.rmtree(temp_dir)

    def _parse_dependencies(self, depends_str: str) -> None:
        """Разбор строки зависимостей"""
        for dep in depends_str.split(','):
            dep = dep.strip()
            if not dep:
                continue
                
            # Извлекаем имя пакета и версионные ограничения
            match = re.match(r'([^\s\(]+)(?:\s*\((.*?)\))?', dep)
            if match:
                pkg_name = match.group(1)
                version_info = match.group(2)
                self.dependencies[pkg_name] = version_info or ''

    def get_installed_files(self) -> List[str]:
        """
        Получение списка файлов, которые будут установлены
        
        Returns:
            List[str]: Список путей файлов
        """
        data_archive = self.get_archive_path('data')
        result = subprocess.run(
            ['tar', 'tf', data_archive],
            capture_output=True,
            text=True,
            check=True
        )
        
        return [
            os.path.join('/', f.replace('./', '', 1))
            for f in result.stdout.splitlines()
        ]

    def install_files(self) -> None:
        """Установка файлов из пакета"""
        data_archive = self.get_archive_path('data')
        try:
            subprocess.run(
                ['tar', 'xf', data_archive, '-C', '/'],
                check=True,
                capture_output=True
            )
            self.files = self.get_installed_files()
        except subprocess.CalledProcessError as e:
            raise PackageError(f"Ошибка при установке файлов: {e.stderr.decode()}")

    def run_maintainer_script(self, script: str) -> None:
        """
        Запуск maintainer скрипта
        
        Args:
            script: Имя скрипта (preinst, postinst, prerm, postrm)
        """
        if script not in self.maintainer_scripts:
            return
            
        script_content = self.maintainer_scripts[script]
        script_path = os.path.join(self.temp_dir, f"{script}.sh")
        
        try:
            # Записываем скрипт во временный файл
            with open(script_path, 'w') as f:
                f.write(script_content)
            os.chmod(script_path, 0o755)
            
            # Запускаем скрипт
            subprocess.run(
                [script_path, 'install'],
                check=True,
                capture_output=True
            )
        except Exception as e:
            raise PackageError(f"Ошибка выполнения {script}: {e}")
        finally:
            if os.path.exists(script_path):
                os.unlink(script_path)

if __name__ == "__main__":
    print("Этот модуль не предназначен для прямого запуска")
    sys.exit(1)