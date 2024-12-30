use std::process::{Command, Stdio};
use std::io::{self, BufRead, BufReader};
use std::env;
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

        // Проверка расширения файла
        if !package.ends_with(".deb") {
            eprintln!("Ошибка: Тип файла пока не поддерживается. Поддерживаются только файлы с расширением .deb");
            return Ok(());
        }
    }

    // Формирование команды для запуска Python-скрипта
    let python_command = Command::new("python3") // или Command::new("python") ???, потом сделаю поддержку двоих
        .arg("deb/main.py")
        .args(&args)
        .stdout(Stdio::piped())
        .stderr(Stdio::piped())
        .spawn()?;

    // Поток для обработки stdout
    let stdout = python_command.stdout.unwrap();
    let stdout_reader = BufReader::new(stdout);
    let stdout_thread = thread::spawn(move || {
        for line in stdout_reader.lines() {
            if let Ok(line) = line {
                println!("{}", line);
            }
        }
    });

    // Поток для обработки stderr
    let stderr = python_command.stderr.unwrap();
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

    Ok(())
}
