package config

type Config struct {
	DBUrl           string `json:"db_url"`
	CurrentUserName string `json:"current_user_name"`
}

func Read() Config {

}

const configFileName = ".gatorconfig.json"

func getConfigFilePath() (string, error) {
	return "", nil
}
func write(cfg Config) error {
	return nil
}
