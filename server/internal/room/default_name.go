package room

import "fmt"

var roomWord = map[string]string{"en": "Room", "hr": "Soba", "sr": "Soba", "mk": "Соба"}

// defaultRoomName names a room after its host, in the host's language, numbered from 2 on later attempts.
func defaultRoomName(username, language string, attempt int) string {
	word, ok := roomWord[language]
	if !ok {
		word = roomWord["en"]
	}
	name := fmt.Sprintf("%s - %s", word, username)
	if attempt > 1 {
		name = fmt.Sprintf("%s %d", name, attempt)
	}
	return name
}
