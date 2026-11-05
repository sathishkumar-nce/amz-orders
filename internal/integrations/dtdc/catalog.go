package dtdc

// Snapshot of the commodity catalogue linked by DTDC Order Upload API v2.0.
// Source: https://docs.google.com/spreadsheets/d/158LuKmF8mHXSQfXcSE-U_NVeUpz-O1LuNlc1ualKEeI/edit
// Retrieved 2026-11-03. IDs are strings in the booking API.
type CommodityOption struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

var commodityOptions = []CommodityOption{
	{ID: "1", Name: "LAPTOP"},
	{ID: "2", Name: "MOBILE"},
	{ID: "3", Name: "CAMERA"},
	{ID: "4", Name: "LAPTOP KIT (LAPTOP, BATTERY, MOUSE, POWER CORD, CA"},
	{ID: "5", Name: "PEN DRIVE / USB STORAGE"},
	{ID: "6", Name: "SLIPPERS"},
	{ID: "7", Name: "OTHERS"},
	{ID: "8", Name: "LAPTOP AND BATTERY"},
	{ID: "9", Name: "POWER CORD"},
	{ID: "10", Name: "OXYGEN CONCENTRATOR"},
	{ID: "11", Name: "BATTERY"},
	{ID: "12", Name: "COMPUTER MOUSE"},
	{ID: "13", Name: "MASKS"},
	{ID: "14", Name: "SENSORS"},
	{ID: "15", Name: "RAKHI"},
	{ID: "16", Name: "CHEQUE BOOKS"},
	{ID: "17", Name: "ARTIFICIAL JEWELLERY"},
	{ID: "18", Name: "BROCHURES"},
	{ID: "19", Name: "CHARGER"},
	{ID: "20", Name: "AUTO / MACHINE PARTS"},
	{ID: "21", Name: "EARPHONES / HEADPHONES / EARPODS"},
	{ID: "22", Name: "LUGGAGE"},
	{ID: "23", Name: "USB CABLE"},
	{ID: "24", Name: "SEEDS"},
	{ID: "25", Name: "LIVE PLANTS"},
	{ID: "26", Name: "PERFUMES"},
	{ID: "27", Name: "LAPTOP BAG / HANDBAG"},
	{ID: "28", Name: "VACCINES"},
	{ID: "29", Name: "POWER BANK"},
	{ID: "32", Name: "UPS"},
	{ID: "33", Name: "CDs / DVDs"},
	{ID: "35", Name: "CHOCOLATES"},
	{ID: "37", Name: "TOYS"},
	{ID: "38", Name: "CLOTHING"},
	{ID: "39", Name: "CHEMICAL LIQUID"},
	{ID: "40", Name: "ELECTRIC ITEMS"},
	{ID: "42", Name: "FURNITURE"},
	{ID: "43", Name: "HOUSEHOLD GOODS"},
	{ID: "44", Name: "CORPORATE GIFTS (EG: MOMENTOES/ WOODEN PLAQUES)"},
	{ID: "45", Name: "PAINTING / ARTWORK"},
	{ID: "46", Name: "SIM CARD / SD CARD"},
	{ID: "47", Name: "HOME APPLIANCES"},
	{ID: "48", Name: "MEDICAL EQUIPMENT"},
	{ID: "49", Name: "FOOD ITEMS"},
	{ID: "50", Name: "PASSPORT"},
	{ID: "51", Name: "GLASS ITEMS"},
	{ID: "52", Name: "TEA BAGS"},
	{ID: "54", Name: "SPICES"},
	{ID: "55", Name: "FRUITS"},
	{ID: "56", Name: "UNSTITCHED FABRIC / SAREE"},
	{ID: "57", Name: "CREDIT / DEBIT CARD"},
	{ID: "59", Name: "PROMOTIONAL MATERIAL (PAPER)"},
	{ID: "60", Name: "MANGOES"},
	{ID: "64", Name: "DESKTOP"},
	{ID: "67", Name: "SHOES"},
	{ID: "70", Name: "CHEMICAL POWDER"},
	{ID: "71", Name: "DRY FRUITS"},
	{ID: "72", Name: "BOOKS"},
	{ID: "73", Name: "PAPERS"},
	{ID: "74", Name: "ELECTRONIC TABLET"},
	{ID: "75", Name: "SWEETS"},
	{ID: "77", Name: "COMPUTER PARTS"},
	{ID: "79", Name: "LED / LCD / CRT TV"},
	{ID: "80", Name: "LED LIGHTS"},
	{ID: "81", Name: "MEDICINE"},
	{ID: "83", Name: "ELECTRONIC ITEMS"},
	{ID: "84", Name: "PLASTIC ITEMS"},
	{ID: "86", Name: "MONITOR"},
	{ID: "87", Name: "STATIONERY"},
	{ID: "88", Name: "MOBILE(WITHOUT BATTERY)"},
	{ID: "89", Name: "PHOTO FRAME"},
	{ID: "90", Name: "FARMING RELATED ITEMS"},
	{ID: "94", Name: "BAGS"},
	{ID: "96", Name: "SURGICAL PARTS"},
	{ID: "152", Name: "LEATHER GOODS"},
	{ID: "153", Name: "PERISHABLE GOODS"},
	{ID: "154", Name: "HOMEOPATHIC MEDICINE"},
	{ID: "155", Name: "DECORATIVE ITEMS"},
	{ID: "156", Name: "CERAMIC GOODS"},
	{ID: "157", Name: "CABLES"},
	{ID: "158", Name: "TYRES"},
	{ID: "159", Name: "IRON GOODS"},
	{ID: "160", Name: "SILVER"},
	{ID: "161", Name: "GOLD PLATED MEDALS"},
	{ID: "162", Name: "TROPHIES"},
	{ID: "163", Name: "PRINITED MATERIALS"},
	{ID: "164", Name: "CAMERA LENS"},
}

// These are documented examples, not an account entitlement list.
var documentedServiceTypes = []string{"B2C PREMIUM", "B2C PRIORITY"}

func commodityName(value string) string {
	for _, option := range commodityOptions {
		if option.ID == value || option.Name == value {
			return option.Name
		}
	}
	return value
}
