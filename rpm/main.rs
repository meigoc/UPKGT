
// код пока не рабочий, в разработке!!!

use std::fs::{self, File};
use std::io::{Read, Write};
use std::path::{Path, PathBuf};
use std::process::Command;

fn extract_rpm_cpio(rpm_path: &str, extract_to: &str) -> Result<(), String> {
    let output = Command::new("rpm2cpio")
        .arg(rpm_path)
        .output()
        .map_err(|e| format!("Ошибка при вызове rpm2cpio: {}", e))?;
    if !output.status.success() {
        return Err(format!("Ошибка при распаковке rpm файла с помощью rpm2cpio: {}", String::from_utf8_lossy(&output.stderr)));
    }
    let cpio_path = Path::new(extract_to).join("package.cpio");
    let mut cpio_file = File::create(&cpio_path)
        .map_err(|e| format!("Не удалось создать файл .cpio: {}", e))?;
    cpio_file.write_all(&output.stdout)
        .map_err(|e| format!("Не удалось записать данные .cpio: {}", e))?;
    let output = Command::new("cpio")
        .arg("-idmv")
        .arg("-D")
        .arg(extract_to)
        .arg("<")
        .arg(cpio_path)
        .output()
        .map_err(|e| format!("Ошибка при распаковке cpio архива: {}", e))?;
    if !output.status.success() {
        return Err(format!("Ошибка при распаковке cpio архива: {}", String::from_utf8_lossy(&output.stderr)));
    }
    Ok(())
}

fn install_package(rpm_path: &str) -> Result<(), String> {
    let extract_dir = "/tmp/rpm_extract";
    extract_rpm_cpio(rpm_path, extract_dir)?;
    let install_dir = Path::new(extract_dir).join("usr");
    if !install_dir.exists() {
        return Err(format!("Не удалось найти директорию установки в архиве: {:?}", install_dir));
    }
    let target_dir = Path::new("/usr");
    if !target_dir.exists() {
        fs::create_dir_all(target_dir).map_err(|e| format!("Ошибка при создании директории: {}", e))?;
    }
    for entry in fs::read_dir(&install_dir).map_err(|e| format!("Ошибка при чтении директории: {}", e))? {
        let entry = entry.map_err(|e| format!("Ошибка при чтении записи директории: {}", e))?;
        let entry_path = entry.path();
        let target_path = target_dir.join(entry.file_name());
        if target_path.exists() {
            return Err(format!("Файл или директория {:?} уже существует. Конфликт при установке.", target_path));
        }
        fs::create_dir_all(target_path.parent().unwrap())
            .map_err(|e| format!("Ошибка при создании целевой директории: {}", e))?;
        fs::copy(entry_path, target_path)
            .map_err(|e| format!("Ошибка при копировании файла: {}", e))?;
    }
    println!("Пакет успешно установлен в {:?}", target_dir);
    Ok(())
}

fn main() {
    let rpm_path = "путь к пакету";
    if let Err(e) = install_package(rpm_path) {
        eprintln!("Произошла ошибка при установке пакета: {}", e);
    }
}
