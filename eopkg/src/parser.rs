use quick_xml::de::from_str;
use serde::Deserialize;
use std::fs;

#[derive(Debug, Deserialize)]
pub struct File {
    pub Path: String,
    pub Type: String,
    pub Size: u64,
    pub Uid: u32,
    pub Gid: u32,
    pub Mode: String,
    pub Hash: String,
}

#[derive(Debug, Deserialize)]
pub struct Files {
    pub File: Vec<File>,
}

#[derive(Debug, Deserialize)]
pub struct Metadata {
    pub Name: String,
    pub Summary: String,
    pub Description: String,
    pub Architecture: String,
}

pub fn parse_files_xml(file_path: &str) -> Result<Files, quick_xml::Error> {
    let content = fs::read_to_string(file_path).expect("Failed to read files.xml");
    from_str(&content)
}

pub fn parse_metadata_xml(file_path: &str) -> Result<Metadata, quick_xml::Error> {
    let content = fs::read_to_string(file_path).expect("Failed to read metadata.xml");
    from_str(&content)
}
