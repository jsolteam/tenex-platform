package migrator

func ParseFilenameExported(name string) (int, string, error) {
	return parseFilename(name)
}

func ChecksumExported(content []byte) string {
	return checksum(content)
}
