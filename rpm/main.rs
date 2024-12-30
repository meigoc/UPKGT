use std::fs::{self, File};
use std::io::{Write};
use std::path::{Path, PathBuf};
use std::process::{Command, Stdio};

fn extract_rpm_cpio(rpm_path: &str, extract_to: &str) -> Result<(), String> {
    fs::create_dir_all(extract_to).map_err(|e| format!("Ошибка при создании директории для извлечения: {}", e))?;
    let rpm2cpio_output = Command::new("rpm2cpio")
        .arg(rpm_path)
        .stdout(Stdio::piped())
        .spawn()
        .map_err(|e| format!("Ошибка при вызове rpm2cpio: {}", e))?;

    let mut cpio = Command::new("cpio")
        .arg("-idmv")
        .current_dir(extract_to)
        .stdin(Stdio::from(rpm2cpio_output.stdout.unwrap()))
        .spawn()
        .map_err(|e| format!("Ошибка при вызове cpio: {}", e))?;

    let status = cpio.wait().map_err(|e| format!("Ошибка при ожидании завершения cpio: {}", e))?;
    if !status.success() {
        return Err("Ошибка при извлечении cpio архива.".to_string());
    }

    Ok(())
}

fn install_package(rpm_path: &str) -> Result<(), String> {
    let extract_dir = "/tmp/rpm_extract";
    let target_dir = Path::new("/usr");


    extract_rpm_cpio(rpm_path, extract_dir)?;
    
    let install_dir = Path::new(extract_dir).join("usr");
    if !install_dir.exists() {
        return Err(format!("Не удалось найти директорию установки: {:?}", install_dir));
    }

    for entry in fs::read_dir(&install_dir).map_err(|e| format!("Ошибка при чтении директории: {}", e))? {
        let entry = entry.map_err(|e| format!("Ошибка при чтении записи директории: {}", e))?;
        let entry_path = entry.path();
        let relative_path = entry_path.strip_prefix(&install_dir).unwrap();
        let target_path = target_dir.join(relative_path);

        if target_path.exists() {
            eprintln!("Предупреждение: файл или директория {:?} уже существует. Пропуск.", target_path);
            continue;
        }

        if entry_path.is_dir() {
            fs::create_dir_all(&target_path)
                .map_err(|e| format!("Ошибка при создании директории: {}", e))?;
        } else {
            if let Some(parent) = target_path.parent() {
                fs::create_dir_all(parent)
                    .map_err(|e| format!("Ошибка при создании директории: {}", e))?;
            }
            fs::copy(&entry_path, &target_path)
                .map_err(|e| format!("Ошибка при копировании файла: {}", e))?;
        }
    }
    
    fs::remove_dir_all(extract_dir).map_err(|e| format!("Ошибка при удалении временной директории: {}", e))?;

    println!("Пакет успешно установлен в {:?}", target_dir);
    Ok(())
}

fn main() {
    let rpm_path = "путь_к_пакету.rpm";

    if let Err(e) = install_package(rpm_path) {
        eprintln!("Произошла ошибка при установке пакета: {}", e);
    }
}
