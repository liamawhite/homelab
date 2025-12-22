package raspberry

import _ "embed"

//go:embed config.txt
var ConfigTxt string

//go:embed eeprom.conf
var EepromConf string
