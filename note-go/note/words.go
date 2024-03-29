// Copyright 2020 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package note

import (
	"hash/fnv"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// Word index data structure
type Word struct {
	WordIndex uint
}

var (
	sortedWords            []Word
	sortedWordsInitialized = false
	sortedWordsInitLock    sync.RWMutex
)

// Class used to sort an index of words
type byWord []Word

func (a byWord) Len() int           { return len(a) }
func (a byWord) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byWord) Less(i, j int) bool { return words2048[a[i].WordIndex] < words2048[a[j].WordIndex] }

// WordToNumber converts a single word to a number
func WordToNumber(word string) (num uint, success bool) {
	// Initialize sorted words array if necessary
	if !sortedWordsInitialized {
		sortedWordsInitLock.Lock()
		if !sortedWordsInitialized {

			// Init the index array
			sortedWords = make([]Word, 2048)
			for i := 0; i < 2048; i++ {
				sortedWords[i].WordIndex = uint(i)
			}

			// Sort the array
			sort.Sort(byWord(sortedWords))

			// We're now initialized
			sortedWordsInitialized = true
		}
		sortedWordsInitLock.Unlock()
	}

	// First normalize the word
	word = strings.ToLower(word)

	// Do a binary chop to find the word or its insertion slot
	i := sort.Search(2048, func(i int) bool { return words2048[sortedWords[i].WordIndex] >= word })

	// Exit if found.  (If we failed to match the result, it's an insertion slot.)
	if i < 2048 && words2048[sortedWords[i].WordIndex] == word {
		return sortedWords[i].WordIndex, true
	}

	return 0, false
}

// WordsToNumber looks up a number from two or three simple words
func WordsToNumber(words string) (num uint32, found bool) {
	var left, middle, right uint
	var success bool

	// For convenience, if a number is supplied just return that number.  I do this so
	// that you can use this same method to parse either a number or the words to get that number.
	word := strings.Split(words, "-")
	if len(word) == 1 {

		// See if this parses cleanly as a number
		i64, err := strconv.ParseUint(words, 10, 32)
		if err == nil {
			return uint32(i64), true
		}
		return 0, false
	}

	// Convert two or three words to numbers, msb to lsb
	if len(word) == 2 {
		middle, success = WordToNumber(word[0])
		if !success {
			return 0, false
		}
		right, success = WordToNumber(word[1])
		if !success {
			return 0, false
		}
	} else {
		left, success = WordToNumber(word[0])
		if !success {
			return 0, false
		}
		middle, success = WordToNumber(word[1])
		if !success {
			return 0, false
		}
		right, success = WordToNumber(word[2])
		if !success {
			return 0, false
		}
	}

	// Map back to bit fields
	result := uint32(left) << 22
	result |= uint32(middle) << 11
	result |= uint32(right)

	return result, true
}

// WordsFromString hashes a string with a 32-bit function and converts it to three simple words
func WordsFromString(in string) (out string) {
	hash := fnv.New32a()
	inbytes := []byte(in)
	hash.Write(inbytes)
	hashval := hash.Sum32()
	out = WordsFromNumber(hashval)
	return
}

// WordsFromNumber converts a number to three simple words
func WordsFromNumber(number uint32) string {
	// Break the 32-bit uint down into 3 bit fields
	left := (number >> 22) & 0x000003ff
	middle := (number >> 11) & 0x000007ff
	right := number & 0x000007ff

	// If the high order is 0, which is frequently the case, just use two words
	if left == 0 {
		return words2048[middle] + "-" + words2048[right]
	}
	return words2048[left] + "-" + words2048[middle] + "-" + words2048[right]
}

// 2048 words, ORDERED but alphabetically unsorted
var words2048 = []string{
	"act",
	"add",
	"age",
	"ago",
	"point",
	"big",
	"all",
	"and",
	"any",
	"arm",
	"art",
	"ash",
	"ask",
	"bad",
	"bag",
	"ban",
	"bar",
	"bat",
	"bay",
	"bed",
	"bee",
	"beg",
	"bet",
	"bid",
	"air",
	"bit",
	"bow",
	"box",
	"boy",
	"bug",
	"bus",
	"buy",
	"cab",
	"can",
	"cap",
	"car",
	"cat",
	"cop",
	"cow",
	"cry",
	"cue",
	"cup",
	"cut",
	"dad",
	"day",
	"die",
	"dig",
	"dip",
	"dog",
	"dot",
	"dry",
	"due",
	"ear",
	"eat",
	"egg",
	"ego",
	"end",
	"era",
	"etc",
	"eye",
	"fan",
	"far",
	"fat",
	"fee",
	"few",
	"fit",
	"fix",
	"fly",
	"fog",
	"for",
	"fun",
	"fur",
	"gap",
	"gas",
	"get",
	"gun",
	"gut",
	"guy",
	"gym",
	"hat",
	"hay",
	"her",
	"hey",
	"him",
	"hip",
	"his",
	"hit",
	"hot",
	"how",
	"hug",
	"huh",
	"ice",
	"its",
	"jar",
	"jaw",
	"jet",
	"job",
	"joy",
	"key",
	"kid",
	"kit",
	"lab",
	"lap",
	"law",
	"leg",
	"let",
	"lid",
	"lie",
	"lip",
	"log",
	"lot",
	"low",
	"mars",
	"mango",
	"map",
	"may",
	"mix",
	"mom",
	"mud",
	"net",
	"new",
	"nod",
	"not",
	"now",
	"nut",
	"oak",
	"odd",
	"off",
	"oil",
	"old",
	"one",
	"our",
	"out",
	"owe",
	"own",
	"pad",
	"pan",
	"pat",
	"pay",
	"pen",
	"pet",
	"pie",
	"pig",
	"pin",
	"pit",
	"pop",
	"pot",
	"put",
	"rat",
	"raw",
	"red",
	"rib",
	"rid",
	"rip",
	"row",
	"run",
	"say",
	"see",
	"set",
	"she",
	"shy",
	"sir",
	"sit",
	"six",
	"ski",
	"sky",
	"son",
	"spy",
	"sum",
	"sun",
	"tag",
	"tap",
	"tax",
	"tea",
	"ten",
	"the",
	"tie",
	"tip",
	"toe",
	"top",
	"toy",
	"try",
	"two",
	"use",
	"van",
	"war",
	"way",
	"web",
	"who",
	"why",
	"win",
	"wow",
	"yes",
	"yet",
	"you",
	"able",
	"acid",
	"aide",
	"ally",
	"also",
	"amid",
	"area",
	"army",
	"atop",
	"aunt",
	"auto",
	"away",
	"baby",
	"back",
	"bake",
	"ball",
	"band",
	"bank",
	"bare",
	"barn",
	"base",
	"bath",
	"beam",
	"bean",
	"bear",
	"beat",
	"beef",
	"beer",
	"bell",
	"belt",
	"bend",
	"best",
	"bias",
	"bike",
	"bill",
	"bind",
	"bird",
	"bite",
	"blue",
	"boat",
	"body",
	"boil",
	"bold",
	"bolt",
	"bomb",
	"bond",
	"bone",
	"book",
	"boom",
	"boot",
	"born",
	"boss",
	"both",
	"bowl",
	"buck",
	"bulb",
	"bulk",
	"bull",
	"burn",
	"bury",
	"bush",
	"busy",
	"cage",
	"cake",
	"call",
	"calm",
	"camp",
	"card",
	"care",
	"cart",
	"case",
	"cash",
	"cast",
	"cave",
	"cell",
	"chef",
	"chew",
	"chin",
	"chip",
	"chop",
	"cite",
	"city",
	"clay",
	"clip",
	"club",
	"clue",
	"coal",
	"coat",
	"code",
	"coin",
	"cold",
	"come",
	"cook",
	"cool",
	"cope",
	"copy",
	"cord",
	"core",
	"corn",
	"cost",
	"coup",
	"crew",
	"crop",
	"cure",
	"cute",
	"dare",
	"dark",
	"data",
	"date",
	"dawn",
	"dead",
	"deal",
	"dear",
	"debt",
	"deck",
	"deem",
	"deep",
	"deer",
	"deny",
	"desk",
	"diet",
	"dirt",
	"dish",
	"dock",
	"doll",
	"door",
	"dose",
	"down",
	"drag",
	"draw",
	"drop",
	"drug",
	"drum",
	"duck",
	"dumb",
	"dump",
	"dust",
	"duty",
	"each",
	"earn",
	"ease",
	"east",
	"easy",
	"echo",
	"edge",
	"edit",
	"else",
	"even",
	"ever",
	"evil",
	"exam",
	"exit",
	"face",
	"fact",
	"fade",
	"fail",
	"fair",
	"fall",
	"fame",
	"fare",
	"farm",
	"fast",
	"fate",
	"feed",
	"feel",
	"file",
	"fill",
	"film",
	"find",
	"fine",
	"fire",
	"firm",
	"fish",
	"five",
	"flag",
	"flat",
	"flee",
	"flip",
	"flow",
	"fold",
	"folk",
	"food",
	"foot",
	"fork",
	"form",
	"four",
	"free",
	"from",
	"fuel",
	"full",
	"fund",
	"gain",
	"game",
	"gang",
	"gate",
	"gaze",
	"gear",
	"gene",
	"gift",
	"girl",
	"give",
	"glad",
	"goal",
	"goat",
	"gold",
	"golf",
	"good",
	"grab",
	"gray",
	"grin",
	"grip",
	"grow",
	"half",
	"hall",
	"hand",
	"hang",
	"hard",
	"harm",
	"hate",
	"haul",
	"have",
	"head",
	"heal",
	"hear",
	"heat",
	"heel",
	"help",
	"herb",
	"here",
	"hero",
	"hers",
	"hide",
	"high",
	"hike",
	"hill",
	"hint",
	"hire",
	"hold",
	"home",
	"hook",
	"hope",
	"horn",
	"host",
	"hour",
	"huge",
	"hunt",
	"hurt",
	"icon",
	"idea",
	"into",
	"iron",
	"item",
	"jail",
	"jazz",
	"join",
	"joke",
	"jump",
	"jury",
	"just",
	"keep",
	"kick",
	"kilt",
	"kind",
	"king",
	"kiss",
	"knee",
	"know",
	"lack",
	"lake",
	"lamp",
	"land",
	"lane",
	"last",
	"late",
	"lawn",
	"lead",
	"leaf",
	"lean",
	"leap",
	"left",
	"lend",
	"lens",
	"less",
	"life",
	"lift",
	"like",
	"limb",
	"line",
	"link",
	"lion",
	"list",
	"live",
	"load",
	"loan",
	"lock",
	"long",
	"look",
	"loop",
	"loss",
	"lost",
	"lots",
	"loud",
	"love",
	"luck",
	"lung",
	"mail",
	"main",
	"make",
	"mall",
	"many",
	"mark",
	"mask",
	"mass",
	"mate",
	"math",
	"meal",
	"mean",
	"meat",
	"meet",
	"melt",
	"menu",
	"mere",
	"mild",
	"milk",
	"mill",
	"mind",
	"mine",
	"miss",
	"mode",
	"mood",
	"moon",
	"more",
	"most",
	"move",
	"much",
	"must",
	"myth",
	"nail",
	"name",
	"near",
	"neat",
	"neck",
	"need",
	"nest",
	"news",
	"next",
	"nice",
	"nine",
	"none",
	"noon",
	"norm",
	"nose",
	"note",
	"odds",
	"okay",
	"once",
	"only",
	"onto",
	"open",
	"ours",
	"oven",
	"over",
	"pace",
	"pack",
	"page",
	"pain",
	"pair",
	"pale",
	"palm",
	"pant",
	"park",
	"part",
	"pass",
	"past",
	"path",
	"peak",
	"peel",
	"peer",
	"pick",
	"pile",
	"pill",
	"pine",
	"pink",
	"pipe",
	"plan",
	"play",
	"plea",
	"plot",
	"plus",
	"poem",
	"poet",
	"poke",
	"pole",
	"poll",
	"pond",
	"pool",
	"poor",
	"pork",
	"port",
	"pose",
	"post",
	"pour",
	"pray",
	"pull",
	"pump",
	"pure",
	"push",
	"quit",
	"race",
	"rack",
	"rage",
	"rail",
	"rain",
	"rank",
	"rare",
	"rate",
	"read",
	"real",
	"rear",
	"rely",
	"rent",
	"rest",
	"rice",
	"rich",
	"ride",
	"ring",
	"riot",
	"rise",
	"risk",
	"road",
	"rock",
	"role",
	"roll",
	"roof",
	"room",
	"root",
	"rope",
	"rose",
	"ruin",
	"rule",
	"rush",
	"sack",
	"safe",
	"sail",
	"sake",
	"sale",
	"salt",
	"same",
	"sand",
	"save",
	"scan",
	"seal",
	"seat",
	"seed",
	"seek",
	"seem",
	"self",
	"sell",
	"send",
	"sexy",
	"shed",
	"ship",
	"shoe",
	"shop",
	"shot",
	"show",
	"shut",
	"side",
	"sign",
	"silk",
	"sing",
	"sink",
	"site",
	"size",
	"skip",
	"slam",
	"slip",
	"slot",
	"slow",
	"snap",
	"snow",
	"soak",
	"soap",
	"soar",
	"sock",
	"sofa",
	"soft",
	"soil",
	"sole",
	"some",
	"song",
	"soon",
	"sort",
	"soul",
	"soup",
	"spin",
	"spit",
	"spot",
	"star",
	"stay",
	"stem",
	"step",
	"stir",
	"stop",
	"such",
	"suck",
	"suit",
	"sure",
	"swim",
	"tail",
	"take",
	"tale",
	"talk",
	"tall",
	"tank",
	"tape",
	"task",
	"team",
	"tear",
	"teen",
	"tell",
	"tend",
	"tent",
	"term",
	"test",
	"text",
	"than",
	"that",
	"them",
	"then",
	"they",
	"thin",
	"this",
	"thus",
	"tide",
	"tile",
	"till",
	"time",
	"tiny",
	"tire",
	"toll",
	"tone",
	"tool",
	"toss",
	"tour",
	"town",
	"trap",
	"tray",
	"tree",
	"trim",
	"trip",
	"tube",
	"tuck",
	"tune",
	"turn",
	"twin",
	"type",
	"unit",
	"upon",
	"urge",
	"used",
	"user",
	"vary",
	"vast",
	"very",
	"view",
	"vote",
	"wage",
	"wait",
	"wake",
	"walk",
	"wall",
	"want",
	"warn",
	"wash",
	"wave",
	"weak",
	"wear",
	"weed",
	"week",
	"well",
	"west",
	"what",
	"when",
	"whip",
	"whom",
	"wide",
	"wink",
	"wild",
	"will",
	"wind",
	"wine",
	"wing",
	"wipe",
	"wire",
	"wise",
	"wish",
	"with",
	"wolf",
	"word",
	"work",
	"wrap",
	"yard",
	"yeah",
	"year",
	"yell",
	"your",
	"zone",
	"true",
	"about",
	"above",
	"actor",
	"adapt",
	"added",
	"admit",
	"adopt",
	"after",
	"again",
	"agent",
	"agree",
	"ahead",
	"aisle",
	"alarm",
	"album",
	"alien",
	"alike",
	"alive",
	"alley",
	"allow",
	"alone",
	"along",
	"alter",
	"among",
	"angle",
	"ankle",
	"apart",
	"apple",
	"apply",
	"arena",
	"argue",
	"arise",
	"armed",
	"array",
	"arrow",
	"aside",
	"asset",
	"avoid",
	"await",
	"awake",
	"award",
	"aware",
	"basic",
	"beach",
	"beast",
	"begin",
	"being",
	"belly",
	"below",
	"bench",
	"birth",
	"blare",
	"blade",
	"bling",
	"blank",
	"blast",
	"blend",
	"bless",
	"blind",
	"blink",
	"block",
	"blond",
	"blotter",
	"board",
	"boast",
	"bonus",
	"boost",
	"booth",
	"brain",
	"brake",
	"brand",
	"brave",
	"bread",
	"break",
	"brick",
	"bride",
	"brief",
	"bring",
	"broad",
	"brood",
	"brush",
	"buddy",
	"build",
	"bunch",
	"burst",
	"buyer",
	"cabin",
	"cable",
	"candy",
	"cargo",
	"carry",
	"carve",
	"catch",
	"cause",
	"cease",
	"chain",
	"chair",
	"chaos",
	"charm",
	"chart",
	"chase",
	"cheat",
	"check",
	"cheek",
	"cheer",
	"chest",
	"chief",
	"child",
	"chill",
	"chunk",
	"claim",
	"class",
	"clean",
	"clear",
	"clerk",
	"click",
	"cliff",
	"climb",
	"cling",
	"clock",
	"close",
	"cloth",
	"cloud",
	"coach",
	"coast",
	"color",
	"couch",
	"could",
	"count",
	"court",
	"cover",
	"crave",
	"craft",
	"crash",
	"crawl",
	"crater",
	"creek",
	"crime",
	"cross",
	"crowd",
	"crown",
	"crush",
	"curve",
	"cycle",
	"daily",
	"dance",
	"death",
	"debut",
	"delay",
	"dense",
	"depth",
	"diary",
	"dirty",
	"donor",
	"doubt",
	"dough",
	"dozen",
	"draft",
	"drain",
	"drama",
	"dream",
	"dress",
	"dried",
	"drift",
	"drill",
	"drink",
	"drive",
	"drown",
	"drunk",
	"dying",
	"eager",
	"early",
	"earth",
	"salty",
	"elbow",
	"elder",
	"elect",
	"elite",
	"empty",
	"enact",
	"enemy",
	"enjoy",
	"enter",
	"entry",
	"equal",
	"equip",
	"erase",
	"essay",
	"event",
	"every",
	"exact",
	"exist",
	"extra",
	"faint",
	"faith",
	"fatal",
	"fault",
	"favor",
	"fence",
	"fever",
	"fewer",
	"fiber",
	"field",
	"fifth",
	"fifty",
	"fight",
	"final",
	"first",
	"fixed",
	"flame",
	"flash",
	"fleet",
	"flesh",
	"float",
	"flood",
	"floor",
	"flour",
	"fluid",
	"focus",
	"force",
	"forth",
	"forty",
	"forum",
	"found",
	"frame",
	"fraud",
	"fresh",
	"front",
	"frown",
	"fruit",
	"fully",
	"funny",
	"genre",
	"ghost",
	"giant",
	"given",
	"glass",
	"globe",
	"glory",
	"glove",
	"grace",
	"grade",
	"grain",
	"grand",
	"grant",
	"grape",
	"grasp",
	"grass",
	"gravel",
	"great",
	"green",
	"greet",
	"grief",
	"gross",
	"group",
	"guard",
	"guess",
	"guest",
	"guide",
	"guilt",
	"habit",
	"happy",
	"harsh",
	"heart",
	"heavy",
	"hello",
	"hence",
	"honey",
	"honor",
	"horse",
	"hotel",
	"house",
	"human",
	"humor",
	"hurry",
	"ideal",
	"image",
	"imply",
	"index",
	"inner",
	"input",
	"irony",
	"issue",
	"jeans",
	"joint",
	"judge",
	"juice",
	"juror",
	"kneel",
	"kayak",
	"knock",
	"known",
	"label",
	"labor",
	"large",
	"laser",
	"later",
	"laugh",
	"layer",
	"learn",
	"least",
	"leave",
	"legal",
	"lemon",
	"level",
	"light",
	"limit",
	"liver",
	"lobby",
	"local",
	"logic",
	"loose",
	"lover",
	"lower",
	"loyal",
	"lucky",
	"lunch",
	"magic",
	"major",
	"maker",
	"march",
	"match",
	"maybe",
	"mayor",
	"medal",
	"media",
	"merit",
	"metal",
	"meter",
	"midst",
	"might",
	"minor",
	"mixed",
	"model",
	"month",
	"moral",
	"motor",
	"mount",
	"mouse",
	"mouth",
	"movie",
	"music",
	"naked",
	"olive",
	"cricket",
	"nerve",
	"never",
	"jade",
	"night",
	"noise",
	"north",
	"novel",
	"nurse",
	"occur",
	"ocean",
	"offer",
	"often",
	"onion",
	"opera",
	"orbit",
	"order",
	"other",
	"ought",
	"outer",
	"owner",
	"paint",
	"panel",
	"panic",
	"paper",
	"party",
	"pasta",
	"patch",
	"pause",
	"phase",
	"phone",
	"photo",
	"piano",
	"piece",
	"pilot",
	"pitch",
	"pizza",
	"place",
	"plain",
	"plant",
	"plate",
	"plead",
	"aim",
	"porch",
	"pound",
	"power",
	"press",
	"price",
	"pride",
	"prime",
	"print",
	"prior",
	"prize",
	"proof",
	"proud",
	"prove",
	"pulse",
	"punch",
	"purse",
	"quest",
	"quick",
	"quiet",
	"quite",
	"quote",
	"radar",
	"radio",
	"raise",
	"rally",
	"ranch",
	"range",
	"rapid",
	"ratio",
	"reach",
	"react",
	"ready",
	"realm",
	"rebel",
	"refer",
	"relax",
	"reply",
	"rider",
	"ridge",
	"rifle",
	"right",
	"risky",
	"rival",
	"river",
	"robot",
	"round",
	"route",
	"royal",
	"rumor",
	"rural",
	"salad",
	"sales",
	"sauce",
	"scale",
	"scare",
	"scene",
	"scent",
	"scope",
	"score",
	"screw",
	"seize",
	"sense",
	"serve",
	"seven",
	"shade",
	"shake",
	"shall",
	"shame",
	"shape",
	"share",
	"shark",
	"sharp",
	"sheep",
	"sheer",
	"sheet",
	"shelf",
	"shell",
	"shift",
	"shirt",
	"shock",
	"shoot",
	"shore",
	"short",
	"shout",
	"shove",
	"shrug",
	"sight",
	"silly",
	"since",
	"sixth",
	"skill",
	"skirt",
	"skull",
	"slave",
	"sleep",
	"slice",
	"slide",
	"slope",
	"small",
	"smart",
	"smell",
	"smile",
	"smoke",
	"snake",
	"sneak",
	"solar",
	"solid",
	"solve",
	"sorry",
	"sound",
	"south",
	"space",
	"spare",
	"spark",
	"speak",
	"speed",
	"spell",
	"spend",
	"spill",
	"spine",
	"spite",
	"split",
	"spoon",
	"sport",
	"spray",
	"squad",
	"stack",
	"staff",
	"stage",
	"stair",
	"stake",
	"stand",
	"stare",
	"start",
	"state",
	"steak",
	"steam",
	"steel",
	"steep",
	"steer",
	"stick",
	"stiff",
	"still",
	"stock",
	"stone",
	"store",
	"storm",
	"story",
	"stove",
	"straw",
	"strip",
	"study",
	"stuff",
	"style",
	"sugar",
	"suite",
	"sunny",
	"super",
	"sweat",
	"sweep",
	"sweet",
	"swell",
	"swing",
	"sword",
	"table",
	"taste",
	"teach",
	"thank",
	"their",
	"theme",
	"there",
	"these",
	"thick",
	"thigh",
	"thing",
	"think",
	"third",
	"those",
	"three",
	"throw",
	"thumb",
	"tight",
	"tired",
	"title",
	"today",
	"tooth",
	"topic",
	"total",
	"touch",
	"tough",
	"towel",
	"tower",
	"trace",
	"track",
	"trade",
	"trail",
	"train",
	"trait",
	"treat",
	"trend",
	"trial",
	"tribe",
	"trick",
	"troop",
	"truck",
	"truly",
	"trunk",
	"trust",
	"truth",
	"tumor",
	"twice",
	"twist",
	"uncle",
	"under",
	"union",
	"unite",
	"unity",
	"until",
	"upper",
	"upset",
	"urban",
	"usual",
	"valid",
	"value",
	"video",
	"virus",
	"visit",
	"vital",
	"vocal",
	"voice",
	"voter",
	"wagon",
	"waist",
	"waste",
	"watch",
	"water",
	"weave",
	"weigh",
	"weird",
	"whale",
	"wheat",
	"wheel",
	"where",
	"which",
	"while",
	"whoop",
	"whole",
	"whose",
	"wider",
	"worm",
	"works",
	"world",
	"worry",
	"worth",
	"would",
	"wound",
	"wrist",
	"write",
	"wrong",
	"yield",
	"young",
	"yours",
	"youth",
	"false",
	"abroad",
	"absorb",
	"accent",
	"accept",
	"access",
	"accuse",
	"across",
	"action",
	"active",
	"actual",
	"adjust",
	"admire",
	"affect",
	"afford",
	"agency",
	"agenda",
	"almost",
	"always",
	"amount",
	"animal",
	"annual",
	"answer",
	"anyone",
	"anyway",
	"appear",
	"around",
	"arrest",
	"arrive",
	"artist",
	"aspect",
	"assert",
	"assess",
	"assign",
	"assist",
	"assume",
	"assure",
	"attach",
	"attack",
	"attend",
	"author",
	"ballot",
	"banana",
	"banker",
	"barrel",
	"basket",
	"battle",
	"beauty",
	"become",
	"before",
	"behalf",
	"behave",
	"behind",
	"belief",
	"belong",
	"beside",
	"better",
	"beyond",
	"bitter",
	"bloody",
	"border",
	"borrow",
	"bottle",
	"bounce",
	"branch",
	"breath",
	"breeze",
	"bridge",
	"bright",
	"broken",
	"broker",
	"bronze",
	"brutal",
	"bubble",
	"bucket",
	"bullet",
	"bureau",
	"butter",
	"button",
	"camera",
	"campus",
	"candle",
	"canvas",
	"carbon",
	"career",
	"carpet",
	"carrot",
	"casino",
	"casual",
	"cattle",
	"center",
	"change",
	"charge",
	"cheese",
	"choice",
	"choose",
	"circle",
	"client",
	"clinic",
	"closed",
	"closet",
	"coffee",
	"collar",
	"combat",
	"comedy",
	"commit",
	"comply",
	"cookie",
	"corner",
	"cotton",
	"county",
	"cousin",
	"create",
	"credit",
	"crisis",
	"cruise",
	"custom",
	"dancer",
	"danger",
	"deadly",
	"dealer",
	"debate",
	"debris",
	"decade",
	"deeply",
	"defeat",
	"defend",
	"define",
	"degree",
	"depart",
	"depend",
	"depict",
	"deploy",
	"deputy",
	"derive",
	"desert",
	"design",
	"desire",
	"detail",
	"detect",
	"device",
	"devote",
	"differ",
	"dining",
	"dinner",
	"direct",
	"divide",
	"doctor",
	"domain",
	"donate",
	"double",
	"drawer",
	"driver",
	"during",
	"easily",
	"eating",
	"editor",
	"effect",
	"effort",
	"either",
	"eleven",
	"emerge",
	"empire",
	"employ",
	"enable",
	"endure",
	"energy",
	"engage",
	"engine",
	"enough",
	"enroll",
	"ensure",
	"entire",
	"entity",
	"equity",
	"escape",
	"estate",
	"evolve",
	"exceed",
	"except",
	"expand",
	"expect",
	"expert",
	"export",
	"expose",
	"extend",
	"extent",
	"fabric",
	"factor",
	"fairly",
	"family",
	"famous",
	"farmer",
	"faster",
	"father",
	"fellow",
	"fierce",
	"figure",
	"filter",
	"fishy",
	"finish",
	"firmly",
	"fiscal",
	"flavor",
	"flight",
	"flower",
	"flying",
	"follow",
	"forest",
	"forget",
	"formal",
	"format",
	"former",
	"foster",
	"fourth",
	"freely",
	"freeze",
	"friend",
	"frozen",
	"future",
	"galaxy",
	"garage",
	"garden",
	"garlic",
	"gather",
	"gender",
	"genius",
	"gifted",
	"glance",
	"global",
	"golden",
	"ground",
	"growth",
	"guitar",
	"handle",
	"happen",
	"hardly",
	"hazard",
	"health",
	"heaven",
	"height",
	"hidden",
	"highly",
	"hockey",
	"honest",
	"hunger",
	"hungry",
	"hunter",
	"ignore",
	"immune",
	"impact",
	"import",
	"impose",
	"income",
	"indeed",
	"infant",
	"inform",
	"injure",
	"injury",
	"inmate",
	"insect",
	"inside",
	"insist",
	"intact",
	"intend",
	"intent",
	"invent",
	"invest",
	"invite",
	"island",
	"itself",
	"jacket",
	"jungle",
	"junior",
	"ladder",
	"lately",
	"latter",
	"launch",
	"lawyer",
	"leader",
	"league",
	"legacy",
	"legend",
	"length",
	"lesson",
	"letter",
	"likely",
	"liquid",
	"listen",
	"little",
	"living",
	"locate",
	"lovely",
	"mainly",
	"makeup",
	"manage",
	"manual",
	"marble",
	"margin",
	"marine",
	"market",
	"master",
	"matter",
	"medium",
	"member",
	"memory",
	"mentor",
	"merely",
	"method",
	"middle",
	"minute",
	"mirror",
	"mobile",
	"modern",
	"modest",
	"modify",
	"moment",
	"monkey",
	"mostly",
	"mother",
	"motion",
	"motive",
	"museum",
	"mutter",
	"mutual",
	"myself",
	"narrow",
	"nation",
	"native",
	"nature",
	"nearby",
	"nearly",
	"needle",
	"nobody",
	"normal",
	"notice",
	"notion",
	"number",
	"object",
	"obtain",
	"occupy",
	"office",
	"online",
	"oppose",
	"option",
	"orange",
	"origin",
	"others",
	"outfit",
	"outlet",
	"output",
	"oxygen",
	"palace",
	"parade",
	"parent",
	"parish",
	"partly",
	"patent",
	"patrol",
	"patron",
	"pencil",
	"people",
	"pepper",
	"period",
	"permit",
	"person",
	"phrase",
	"pickup",
	"pillow",
	"planet",
	"player",
	"please",
	"plenty",
	"plunge",
	"pocket",
	"poetry",
	"policy",
	"poster",
	"potato",
	"powder",
	"prefer",
	"pretty",
	"priest",
	"profit",
	"prompt",
	"proper",
	"public",
	"purple",
	"pursue",
	"puzzle",
	"rabbit",
	"random",
	"rarely",
	"rather",
	"rating",
	"reader",
	"really",
	"reason",
	"recall",
	"recent",
	"recipe",
	"record",
	"reduce",
	"reform",
	"refuse",
	"regain",
	"regard",
	"regime",
	"region",
	"reject",
	"relate",
	"relief",
	"remain",
	"remark",
	"remind",
	"remote",
	"remove",
	"rental",
	"repair",
	"repeat",
	"report",
	"rescue",
	"resign",
	"resist",
	"resort",
	"result",
	"resume",
	"retail",
	"retain",
	"retire",
	"return",
	"reveal",
	"review",
	"reward",
	"rhythm",
	"ribbon",
	"ritual",
	"rocket",
	"rubber",
	"ruling",
	"runner",
	"safely",
	"safety",
	"salary",
	"salmon",
	"sample",
	"saving",
	"scared",
	"scheme",
	"school",
	"scream",
	"screen",
	"script",
	"search",
	"season",
	"second",
	"secret",
	"sector",
	"secure",
	"seldom",
	"select",
	"seller",
	"senior",
	"sensor",
	"series",
	"settle",
	"severe",
	"shadow",
	"shorts",
	"should",
	"shrimp",
	"signal",
	"silent",
	"silver",
	"simple",
	"simply",
	"singer",
	"single",
	"sister",
	"sleeve",
	"slight",
	"slowly",
	"smooth",
	"soccer",
	"social",
	"sodium",
	"soften",
	"softly",
	"solely",
	"source",
	"speech",
	"sphere",
	"spirit",
	"spread",
	"spring",
	"square",
	"stable",
	"stance",
	"statue",
	"status",
	"steady",
	"strain",
	"streak",
	"stream",
	"street",
	"stress",
	"strict",
	"strike",
	"string",
	"stroke",
	"strong",
	"studio",
	"stupid",
	"submit",
	"subtle",
	"suburb",
	"sudden",
	"suffer",
	"summer",
	"summit",
	"supply",
	"surely",
	"survey",
	"switch",
	"symbol",
	"system",
	"tackle",
	"tactic",
	"talent",
	"target",
	"temple",
	"tender",
	"tennis",
	"thanks",
	"theory",
	"thirty",
	"though",
	"thread",
	"thrive",
	"throat",
	"ticket",
	"timber",
	"timing",
	"tissue",
	"toilet",
	"tomato",
	"tonic",
	"toward",
	"tragic",
	"trauma",
	"travel",
	"treaty",
	"tribal",
	"tunnel",
	"turkey",
	"twelve",
	"twenty",
	"unfair",
	"unfold",
	"unique",
	"unless",
	"unlike",
	"update",
	"useful",
	"vacuum",
	"valley",
	"vanish",
	"vendor",
	"verbal",
	"versus",
	"vessel",
	"viewer",
	"virtue",
	"vision",
	"visual",
	"volume",
	"voting",
	"wander",
	"warmth",
	"wealth",
	"weapon",
	"weekly",
	"weight",
	"widely",
	"window",
	"winner",
	"winter",
	"wisdom",
	"within",
	"wonder",
	"wooden",
	"worker",
	"writer",
	"yellow",
}

// end
