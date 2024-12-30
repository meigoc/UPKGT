#!/usr/bin/env python3

"""
upkgt-deb - главный модуль программы
Автор: AnmiTaliDev
Дата: 2024-12-30 18:15:23 UTC
"""

import os
import sys
import logging
import argparse
from typing import List, Optional
from datetime import datetime

from package_manager import PackageManager, PackageManagerError
from package import PackageError
from utils import setup_logging, check_root, check_system_requirements

VERSION = "1.0.0"
DESCRIPTION = """
UPKGT-DEB - Утилита для установки .deb пакетов
Разработано для Arch Linux и других систем, не использующих dpkg
"""

class CommandLineInterface:
    """Класс для обработки командной строки"""
    
    def __init__(self):
        self.parser = self._create_parser()
        self.package_manager = PackageManager()

    def _create_parser(self) -> argparse.ArgumentParser:
        """Создание парсера аргументов командной строки"""
        parser = argparse.ArgumentParser(
            description=DESCRIPTION,
            formatter_class=argparse.RawDescriptionHelpFormatter
        )
        
        parser.add_argument(
            '--version',
            action='version',
            version=f'%(prog)s {VERSION}'
        )
        
        parser.add_argument(
            '--debug',
            action='store_true',
            help='Включить отладочный режим'
        )
        
        subparsers = parser.add_subparsers(
            dest='command',
            required=True,
            help='Доступные команды'
        )
        
        # Команда install
        install_parser = subparsers.add_parser(
            'install',
            help='Установить .deb пакет'
        )
        install_parser.add_argument(
            'package',
            help='Путь к .deb файлу'
        )
        install_parser.add_argument(
            '--force',
            action='store_true',
            help='Принудительная установка'
        )
        
        # Команда remove
        remove_parser = subparsers.add_parser(
            'remove',
            help='Удалить установленный пакет'
        )
        remove_parser.add_argument(
            'package',
            help='Имя пакета для удаления'
        )
        
        # Команда list
        list_parser = subparsers.add_parser(
            'list',
            help='Показать установленные пакеты'
        )
        list_parser.add_argument(
            '--verbose', '-v',
            action='store_true',
            help='Показать подробную информацию'
        )
        
        # Команда verify
        verify_parser = subparsers.add_parser(
            'verify',
            help='Проверить целостность установленных файлов'
        )
        verify_parser.add_argument(
            'package',
            nargs='?',
            help='Имя пакета для проверки (если не указано - все пакеты)'
        )
        
        return parser

    def execute(self, args: Optional[List[str]] = None) -> int:
        """
        Выполнение команды
        
        Args:
            args: Список аргументов командной строки
            
        Returns:
            int: Код возврата (0 - успех, не 0 - ошибка)
        """
        try:
            parsed_args = self.parser.parse_args(args)
            setup_logging(parsed_args.debug)
            
            # Проверяем root права для команд, требующих их
            if parsed_args.command in {'install', 'remove'}:
                check_root()
            
            # Обработка команд
            if parsed_args.command == 'install':
                return self._handle_install(parsed_args)
            elif parsed_args.command == 'remove':
                return self._handle_remove(parsed_args)
            elif parsed_args.command == 'list':
                return self._handle_list(parsed_args)
            elif parsed_args.command == 'verify':
                return self._handle_verify(parsed_args)
            else:
                logging.error(f"Неизвестная команда: {parsed_args.command}")
                return 1
                
        except Exception as e:
            logging.error(str(e))
            if parsed_args.debug:
                logging.exception("Подробности ошибки:")
            return 1

    def _handle_install(self, args) -> int:
        """Обработка команды install"""
        try:
            if not os.path.exists(args.package):
                logging.error(f"Файл не найден: {args.package}")
                return 1
                
            self.package_manager.install(args.package, args.force)
            return 0
            
        except (PackageError, PackageManagerError) as e:
            logging.error(f"Ошибка установки: {e}")
            return 1

    def _handle_remove(self, args) -> int:
        """Обработка команды remove"""
        try:
            self.package_manager.remove(args.package)
            return 0
        except PackageManagerError as e:
            logging.error(f"Ошибка удаления: {e}")
            return 1

    def _handle_list(self, args) -> int:
        """Обработка команды list"""
        try:
            packages = self.package_manager.get_installed_packages()
            if not packages:
                print("Нет установленных пакетов")
                return 0
                
            for name, info in sorted(packages.items()):
                if args.verbose:
                    print(f"\nПакет: {name}")
                    print(f"Версия: {info['version']}")
                    print(f"Установлен: {info['install_date']}")
                    print(f"Описание: {info['description']}")
                    if info['depends']:
                        print("Зависимости:")
                        for dep, ver in info['depends'].items():
                            print(f"  - {dep} {ver}")
                else:
                    print(f"{name} {info['version']}")
                    
            return 0
            
        except PackageManagerError as e:
            logging.error(f"Ошибка получения списка пакетов: {e}")
            return 1

    def _handle_verify(self, args) -> int:
        """Обработка команды verify"""
        try:
            results = self.package_manager.verify_packages(args.package)
            if not results:
                print("Все файлы проверены успешно")
                return 0
                
            print("Обнаружены несоответствия:")
            for path, issue in results.items():
                print(f"{path}: {issue}")
                
            return 1
            
        except PackageManagerError as e:
            logging.error(f"Ошибка проверки: {e}")
            return 1

def main() -> int:
    """Точка входа программы"""
    cli = CommandLineInterface()
    return cli.execute()

if __name__ == "__main__":
    sys.exit(main())