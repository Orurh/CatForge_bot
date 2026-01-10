package narrative

import "catforge/internal/domain"

func pick(list []string, flavor uint16, salt int) string {
	if len(list) == 0 {
		return ""
	}
	i := (int(flavor) + salt) % len(list)
	if i < 0 {
		i = -i
	}
	return list[i]
}

var intros = []string{
	"вышел на охоту —",
	"пошёл охотиться и",
	"выскользнул в ночь —",
	"притаился в тени и",
	"рванул на промысел —",
	"вышел проверить владения и",
	"устроил засаду и",
}

var mice = []string{
	"нашёл стайку мышей",
	"загнал мышей в угол",
	"перехитрил шустрых мышей",
	"поймал сразу две мыши подряд",
	"выследил мышиную тропу",
	"устроил мышам внезапную атаку",
	"поймал жирную мышь у норки",
}

var pigeon = []string{
	"подкараулил голубя у крыши",
	"спугнул птиц и ухватил добычу",
	"поймал наглого голубя у мусорки",
	"перехватил пернатого на взлёте",
	"нашёл гнездо и унёс трофей",
	"устроил пернатым переполох",
}

var lizard = []string{
	"поймал юркую ящерицу",
	"нашёл ящерицу на тёплом камне",
	"перехитрил быструю ящерицу",
	"подкараулил ящерицу в траве",
	"надыбал добычу у забора",
	"поймал хвост — и всё равно доволен",
}

var bigRat = []string{
	"наткнулся на огромную крысу",
	"сцепился с матерой крысой",
	"вызвал на бой крысу-гиганта",
	"поймал здоровенную крысу в подвале",
	"перекрыл крысе путь к бегству",
	"встретил крысу, от которой шарахаются все",
}

// HuntStory returns a short narrative (without cat name), deterministic by flavor.
func HuntStory(enc domain.Encounter, flavor uint16) string {
	intro := pick(intros, flavor, 0)

	var prey string
	switch enc {
	case domain.EncounterBigRat:
		prey = pick(bigRat, flavor, 37)
	case domain.EncounterPigeon:
		prey = pick(pigeon, flavor, 37)
	case domain.EncounterLizard:
		prey = pick(lizard, flavor, 37)
	default:
		prey = pick(mice, flavor, 37)
	}
	return intro + " " + prey
}
