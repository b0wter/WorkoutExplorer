package ingest

import "regexp"

// appleName maps an Apple hardware identifier to its marketing model name
// (decision of 13 Nov 2026, changing OQ-147 for Apple; F-43 DI-2). Apps that export
// Apple Health workouts write "Watch7,5" as the product name, whatever the
// manufacturer field says.
//
// UPDATE BY HAND once a year after Apple's September event: add the new
// Watch and iPhone identifiers. An identifier not in the table still shows
// "Apple Watch" or "iPhone" (never the raw code), so a missed update is harmless.
//
// Facts compiled from the adamawolf "Apple device model identifiers" gist,
// ipsw.me (API) and EveryMac (SE 3 identifiers). Case size, cellular and
// region variants share one name. The table runs through the 2025 models.
func appleName(s string) string {
	if n, ok := appleNames[s]; ok {
		return n
	}
	switch {
	case appleWatchID.MatchString(s):
		return "Apple Watch"
	case appleIPhoneID.MatchString(s):
		return "iPhone"
	}
	return s
}

var (
	appleWatchID  = regexp.MustCompile(`^Watch\d+,\d+$`)
	appleIPhoneID = regexp.MustCompile(`^iPhone\d+,\d+$`)
)

var appleNames = map[string]string{
	"Watch1,1":   "Apple Watch (1st generation)",
	"Watch1,2":   "Apple Watch (1st generation)",
	"Watch2,6":   "Apple Watch Series 1",
	"Watch2,7":   "Apple Watch Series 1",
	"Watch2,3":   "Apple Watch Series 2",
	"Watch2,4":   "Apple Watch Series 2",
	"Watch3,1":   "Apple Watch Series 3",
	"Watch3,2":   "Apple Watch Series 3",
	"Watch3,3":   "Apple Watch Series 3",
	"Watch3,4":   "Apple Watch Series 3",
	"Watch4,1":   "Apple Watch Series 4",
	"Watch4,2":   "Apple Watch Series 4",
	"Watch4,3":   "Apple Watch Series 4",
	"Watch4,4":   "Apple Watch Series 4",
	"Watch5,1":   "Apple Watch Series 5",
	"Watch5,2":   "Apple Watch Series 5",
	"Watch5,3":   "Apple Watch Series 5",
	"Watch5,4":   "Apple Watch Series 5",
	"Watch5,9":   "Apple Watch SE (1st generation)",
	"Watch5,10":  "Apple Watch SE (1st generation)",
	"Watch5,11":  "Apple Watch SE (1st generation)",
	"Watch5,12":  "Apple Watch SE (1st generation)",
	"Watch6,1":   "Apple Watch Series 6",
	"Watch6,2":   "Apple Watch Series 6",
	"Watch6,3":   "Apple Watch Series 6",
	"Watch6,4":   "Apple Watch Series 6",
	"Watch6,6":   "Apple Watch Series 7",
	"Watch6,7":   "Apple Watch Series 7",
	"Watch6,8":   "Apple Watch Series 7",
	"Watch6,9":   "Apple Watch Series 7",
	"Watch6,10":  "Apple Watch SE (2nd generation)",
	"Watch6,11":  "Apple Watch SE (2nd generation)",
	"Watch6,12":  "Apple Watch SE (2nd generation)",
	"Watch6,13":  "Apple Watch SE (2nd generation)",
	"Watch6,14":  "Apple Watch Series 8",
	"Watch6,15":  "Apple Watch Series 8",
	"Watch6,16":  "Apple Watch Series 8",
	"Watch6,17":  "Apple Watch Series 8",
	"Watch6,18":  "Apple Watch Ultra",
	"Watch7,1":   "Apple Watch Series 9",
	"Watch7,2":   "Apple Watch Series 9",
	"Watch7,3":   "Apple Watch Series 9",
	"Watch7,4":   "Apple Watch Series 9",
	"Watch7,5":   "Apple Watch Ultra 2",
	"Watch7,8":   "Apple Watch Series 10",
	"Watch7,9":   "Apple Watch Series 10",
	"Watch7,10":  "Apple Watch Series 10",
	"Watch7,11":  "Apple Watch Series 10",
	"Watch7,12":  "Apple Watch Ultra 3",
	"Watch7,13":  "Apple Watch SE (3rd generation)",
	"Watch7,14":  "Apple Watch SE (3rd generation)",
	"Watch7,15":  "Apple Watch SE (3rd generation)",
	"Watch7,16":  "Apple Watch SE (3rd generation)",
	"Watch7,17":  "Apple Watch Series 11",
	"Watch7,18":  "Apple Watch Series 11",
	"Watch7,19":  "Apple Watch Series 11",
	"Watch7,20":  "Apple Watch Series 11",
	"iPhone1,1":  "iPhone",
	"iPhone1,2":  "iPhone 3G",
	"iPhone2,1":  "iPhone 3GS",
	"iPhone3,1":  "iPhone 4",
	"iPhone3,2":  "iPhone 4",
	"iPhone3,3":  "iPhone 4",
	"iPhone4,1":  "iPhone 4S",
	"iPhone5,1":  "iPhone 5",
	"iPhone5,2":  "iPhone 5",
	"iPhone5,3":  "iPhone 5c",
	"iPhone5,4":  "iPhone 5c",
	"iPhone6,1":  "iPhone 5s",
	"iPhone6,2":  "iPhone 5s",
	"iPhone7,1":  "iPhone 6 Plus",
	"iPhone7,2":  "iPhone 6",
	"iPhone8,1":  "iPhone 6s",
	"iPhone8,2":  "iPhone 6s Plus",
	"iPhone8,4":  "iPhone SE (1st generation)",
	"iPhone9,1":  "iPhone 7",
	"iPhone9,2":  "iPhone 7 Plus",
	"iPhone9,3":  "iPhone 7",
	"iPhone9,4":  "iPhone 7 Plus",
	"iPhone10,1": "iPhone 8",
	"iPhone10,2": "iPhone 8 Plus",
	"iPhone10,3": "iPhone X",
	"iPhone10,4": "iPhone 8",
	"iPhone10,5": "iPhone 8 Plus",
	"iPhone10,6": "iPhone X",
	"iPhone11,2": "iPhone XS",
	"iPhone11,4": "iPhone XS Max",
	"iPhone11,6": "iPhone XS Max",
	"iPhone11,8": "iPhone XR",
	"iPhone12,1": "iPhone 11",
	"iPhone12,3": "iPhone 11 Pro",
	"iPhone12,5": "iPhone 11 Pro Max",
	"iPhone12,8": "iPhone SE (2nd generation)",
	"iPhone13,1": "iPhone 12 mini",
	"iPhone13,2": "iPhone 12",
	"iPhone13,3": "iPhone 12 Pro",
	"iPhone13,4": "iPhone 12 Pro Max",
	"iPhone14,2": "iPhone 13 Pro",
	"iPhone14,3": "iPhone 13 Pro Max",
	"iPhone14,4": "iPhone 13 mini",
	"iPhone14,5": "iPhone 13",
	"iPhone14,6": "iPhone SE (3rd generation)",
	"iPhone14,7": "iPhone 14",
	"iPhone14,8": "iPhone 14 Plus",
	"iPhone15,2": "iPhone 14 Pro",
	"iPhone15,3": "iPhone 14 Pro Max",
	"iPhone15,4": "iPhone 15",
	"iPhone15,5": "iPhone 15 Plus",
	"iPhone16,1": "iPhone 15 Pro",
	"iPhone16,2": "iPhone 15 Pro Max",
	"iPhone17,1": "iPhone 16 Pro",
	"iPhone17,2": "iPhone 16 Pro Max",
	"iPhone17,3": "iPhone 16",
	"iPhone17,4": "iPhone 16 Plus",
	"iPhone17,5": "iPhone 16e",
	"iPhone18,1": "iPhone 17 Pro",
	"iPhone18,2": "iPhone 17 Pro Max",
	"iPhone18,3": "iPhone 17",
	"iPhone18,4": "iPhone Air",
	"iPhone18,5": "iPhone 17e",
}
