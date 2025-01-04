use quick_xml::de::from_str;
use serde::Deserialize;

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
    pub Version: String,
    pub License: String,
}

pub fn parse_files_xml(content: &str) -> Result<Files, quick_xml::Error> {
    from_str(content)
}

pub fn parse_metadata_xml(content: &str) -> Result<Metadata, quick_xml::Error> {
    from_str(content)
}
