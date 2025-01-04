use std::process::{Command, Stdio};
use std::io::{self, BufRead, BufReader};
use std::env;
use std::path::Path;
use std::thread;

fn main() -> io::Result<()> {
    // Сбор аргументов командной строки
    let args: Vec<String> = env::args().skip(1).collect();
    
    // Проверка на наличие обязательного аргумента после команды install
    if let Some((pos, _)) = args.iter().enumerate().find(|&(_, arg)| arg == "install") {
        if pos + 1 >= args.len() {
            eprintln!("Ошибка: Обязательный аргумент 'package' не указан после 'install'");
            return Ok(());
        }

        let package = &args[pos + 1];
        let package_path = Path::new(package);

        // Проверка расширения файла
        if package.ends_with(".deb") {
            handle_deb(&args)?;
        } else if package.ends_with(".eopkg") {
            handle_eopkg(package_path)?;
        } else {
            eprintln!("Ошибка: Тип файла не поддерживается. Поддерживаются только файлы с расширением .deb и .eopkg");
            return Ok(());
        }
    }

    Ok(())
}

fn handle_deb(args: &[String]) -> io::Result<()> {
    let python_command = Command::new("python3")
        .arg("deb/main.py")
        .args(args)
        .stdout(Stdio::piped())
        .stderr(Stdio::piped())
        .spawn()?;

    process_output(python_command)
}

fn handle_eopkg(package_path: &Path) -> io::Result<()> {
    let eopkginst_path = "eopkg/eopkginst"; // должен быть бинарным файлом, а может сделать обязательно cargo и просто запускать не скомпилированный файл?
    if which::which(eopkginst_path).is_err() {
        eprintln!("Ошибка: Бинарный файл '{}' не найден в PATH", eopkginst_path);
        return Ok(());
    }

    println!("Устанавливаем пакет .eopkg: {}", package_path.display());

    // Вызов eopkginst с аргументом пакета
    let eopkg_command = Command::new(eopkginst_path)
        .arg("--install")
        .arg(package_path.to_str().unwrap())
        .stdout(Stdio::piped())
        .stderr(Stdio::piped())
        .spawn()?;

    process_output(eopkg_command)
}

fn process_output(mut command: std::process::Child) -> io::Result<()> {
    // Поток для обработки stdout
    let stdout = command.stdout.take().unwrap();
    let stdout_reader = BufReader::new(stdout);
    let stdout_thread = thread::spawn(move || {
        for line in stdout_reader.lines() {
            if let Ok(line) = line {
                println!("{}", line);
            }
        }
    });

    // Поток для обработки stderr
    let stderr = command.stderr.take().unwrap();
    let stderr_reader = BufReader::new(stderr);
    let stderr_thread = thread::spawn(move || {
        for line in stderr_reader.lines() {
            if let Ok(line) = line {
                eprintln!("{}", line);
            }
        }
    });

    // Ожидание завершения потоков
    stdout_thread.join().unwrap();
    stderr_thread.join().unwrap();

    command.wait()?;
    Ok(())
}
