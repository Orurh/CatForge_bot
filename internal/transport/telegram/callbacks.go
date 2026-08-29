package telegram

import (
	"strconv"
	"strings"
)

// Callback constants (avoid scattered string literals).
const (
	CBPersonalPrefix    = "catui:"
	CBNavMenu           = "nav:menu"
	CBMenuCat           = "menu:cat"
	CBMenuTrain         = "menu:train"
	CBMenuExp           = "menu:exp"
	CBMenuPVP           = "menu:pvp"
	CBResetAsk          = "reset:ask"
	CBResetConfirm      = "reset:confirm"
	CBProfileRefresh    = "profile:refresh"
	CBTrainRefresh      = "train:refresh"
	CBTrainDo           = "train:do"
	CBExpeditionRefresh = "expedition:refresh"
	CBCollection        = "collection:list"
	CBBestiary          = "bestiary:list"
	CBNameAsk           = "name:ask"
	CBNameSkip          = "name:skip"
	CBProfileAutoSpeak  = "profile:autospeak"
	CBProfileHumor      = "profile:humor"
	CBMenuYard          = "menu:yard"
	CBMenuFight         = "menu:fight"
	CBMenuAskCat        = "menu:askcat"
	CBNoop              = "noop"
)

const (
	CBStarterPrefix           = "starter:"
	CBExpeditionChoosePrefix  = "expedition:choose:"
	CBExpeditionDoPrefix      = "expedition:do:"
	CBCollectionItemPrefix    = "collection:item:"
	CBCollectionEquipPrefix   = "collection:equip:"
	CBCollectionUpgradePrefix = "collection:upgrade:"
	CBYardChoicePrefix        = "yard:choice:"
)

func PersonalCallback(ownerUserID int64, action string) string {
	return CBPersonalPrefix + strconv.FormatInt(ownerUserID, 10) + ":" + action
}

func ParsePersonalCallback(data string) (ownerUserID int64, action string, ok bool) {
	if !strings.HasPrefix(data, CBPersonalPrefix) {
		return 0, "", false
	}
	parts := strings.SplitN(strings.TrimPrefix(data, CBPersonalPrefix), ":", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[1]) == "" {
		return 0, "", false
	}
	owner, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || owner <= 0 {
		return 0, "", false
	}
	return owner, parts[1], true
}
