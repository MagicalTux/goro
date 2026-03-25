package standard

import (
	"bytes"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

// > const
const (
	HTML_SPECIALCHARS phpv.ZInt = 0
	HTML_ENTITIES     phpv.ZInt = 1
)

// > const
const (
	ENT_HTML_QUOTE_NONE                  phpv.ZInt = 0
	ENT_HTML_QUOTE_SINGLE                phpv.ZInt = 1
	ENT_HTML_QUOTE_DOUBLE                phpv.ZInt = 2
	ENT_HTML_IGNORE_ERRORS               phpv.ZInt = 4
	ENT_HTML_SUBSTITUTE_ERRORS           phpv.ZInt = 8
	ENT_HTML_DOC_TYPE_MASK               phpv.ZInt = (16 | 32)
	ENT_HTML_DOC_HTML401                 phpv.ZInt = 0
	ENT_HTML_DOC_XML1                    phpv.ZInt = 16
	ENT_HTML_DOC_XHTML                   phpv.ZInt = 32
	ENT_HTML_DOC_HTML5                   phpv.ZInt = (16 | 32)
	ENT_HTML_SUBSTITUTE_DISALLOWED_CHARS phpv.ZInt = 128
	ENT_COMPAT                           phpv.ZInt = ENT_HTML_QUOTE_DOUBLE
	ENT_QUOTES                           phpv.ZInt = (ENT_HTML_QUOTE_DOUBLE | ENT_HTML_QUOTE_SINGLE)
	ENT_NOQUOTES                         phpv.ZInt = ENT_HTML_QUOTE_NONE
	ENT_IGNORE                           phpv.ZInt = ENT_HTML_IGNORE_ERRORS
	ENT_SUBSTITUTE                       phpv.ZInt = ENT_HTML_SUBSTITUTE_ERRORS
	ENT_HTML401                          phpv.ZInt = 0
	ENT_XML1                             phpv.ZInt = 16
	ENT_XHTML                            phpv.ZInt = 32
	ENT_HTML5                            phpv.ZInt = (16 | 32)
	ENT_DISALLOWED                       phpv.ZInt = 128
)

func init() {
	entitySet = make(map[string]struct{}, len(tableBase)+len(table401)+len(tableHtml5))
	for _, t := range [](map[string]string){tableBase, table401, tableHtml5} {
		for _, v := range t {
			entitySet[v] = struct{}{}
		}
	}
	entitySet["&quot;"] = struct{}{}
	entitySet["&#039;"] = struct{}{}
	entitySet["&apos;"] = struct{}{}
}

var entitySet map[string]struct{}

var tableBase = map[string]string{
	"&": "&amp;",
	"<": "&lt;",
	">": "&gt;",
}

var table401 = map[string]string{
	" ": "&nbsp;", "¡": "&iexcl;", "¢": "&cent;", "£": "&pound;", "¤": "&curren;", "¥": "&yen;",
	"¦": "&brvbar;", "§": "&sect;", "¨": "&uml;", "©": "&copy;", "ª": "&ordf;", "«": "&laquo;",
	"¬": "&not;", "­": "&shy;", "®": "&reg;", "¯": "&macr;", "°": "&deg;",
	"±": "&plusmn;", "²": "&sup2;", "³": "&sup3;", "´": "&acute;", "µ": "&micro;",
	"¶": "&para;", "·": "&middot;", "¸": "&cedil;", "¹": "&sup1;", "º": "&ordm;",
	"»": "&raquo;", "¼": "&frac14;", "½": "&frac12;", "¾": "&frac34;", "¿": "&iquest;",
	"À": "&Agrave;", "Á": "&Aacute;", "Â": "&Acirc;", "Ã": "&Atilde;", "Ä": "&Auml;",
	"Å": "&Aring;", "Æ": "&AElig;", "Ç": "&Ccedil;", "È": "&Egrave;", "É": "&Eacute;",
	"Ê": "&Ecirc;", "Ë": "&Euml;", "Ì": "&Igrave;", "Í": "&Iacute;", "Î": "&Icirc;",
	"Ï": "&Iuml;", "Ð": "&ETH;", "Ñ": "&Ntilde;", "Ò": "&Ograve;", "Ó": "&Oacute;",
	"Ô": "&Ocirc;", "Õ": "&Otilde;", "Ö": "&Ouml;", "×": "&times;", "Ø": "&Oslash;",
	"Ù": "&Ugrave;", "Ú": "&Uacute;", "Û": "&Ucirc;", "Ü": "&Uuml;", "Ý": "&Yacute;",
	"Þ": "&THORN;", "ß": "&szlig;", "à": "&agrave;", "á": "&aacute;", "â": "&acirc;",
	"ã": "&atilde;", "ä": "&auml;", "å": "&aring;", "æ": "&aelig;", "ç": "&ccedil;",
	"è": "&egrave;", "é": "&eacute;", "ê": "&ecirc;", "ë": "&euml;", "ì": "&igrave;",
	"í": "&iacute;", "î": "&icirc;", "ï": "&iuml;", "ð": "&eth;", "ñ": "&ntilde;",
	"ò": "&ograve;", "ó": "&oacute;", "ô": "&ocirc;", "õ": "&otilde;", "ö": "&ouml;",
	"÷": "&divide;", "ø": "&oslash;", "ù": "&ugrave;", "ú": "&uacute;", "û": "&ucirc;",
	"ü": "&uuml;", "ý": "&yacute;", "þ": "&thorn;", "ÿ": "&yuml;", "Œ": "&OElig;",
	"œ": "&oelig;", "Š": "&Scaron;", "š": "&scaron;", "Ÿ": "&Yuml;", "ƒ": "&fnof;",
	"ˆ": "&circ;", "˜": "&tilde;", "Α": "&Alpha;", "Β": "&Beta;", "Γ": "&Gamma;",
	"Δ": "&Delta;", "Ε": "&Epsilon;", "Ζ": "&Zeta;", "Η": "&Eta;", "Θ": "&Theta;",
	"Ι": "&Iota;", "Κ": "&Kappa;", "Λ": "&Lambda;", "Μ": "&Mu;", "Ν": "&Nu;",
	"Ξ": "&Xi;", "Ο": "&Omicron;", "Π": "&Pi;", "Ρ": "&Rho;", "Σ": "&Sigma;",
	"Τ": "&Tau;", "Υ": "&Upsilon;", "Φ": "&Phi;", "Χ": "&Chi;", "Ψ": "&Psi;",
	"Ω": "&Omega;", "α": "&alpha;", "β": "&beta;", "γ": "&gamma;", "δ": "&delta;",
	"ε": "&epsilon;", "ζ": "&zeta;", "η": "&eta;", "θ": "&theta;", "ι": "&iota;",
	"κ": "&kappa;", "λ": "&lambda;", "μ": "&mu;", "ν": "&nu;", "ξ": "&xi;",
	"ο": "&omicron;", "π": "&pi;", "ρ": "&rho;", "ς": "&sigmaf;", "σ": "&sigma;",
	"τ": "&tau;", "υ": "&upsilon;", "φ": "&phi;", "χ": "&chi;", "ψ": "&psi;",
	"ω": "&omega;", "ϑ": "&thetasym;", "ϒ": "&upsih;", "ϖ": "&piv;", " ": "&ensp;",
	" ": "&emsp;", " ": "&thinsp;", "‌": "&zwnj;", "‍": "&zwj;", "‎": "&lrm;",
	"‏": "&rlm;", "–": "&ndash;", "—": "&mdash;", "‘": "&lsquo;", "’": "&rsquo;",
	"‚": "&sbquo;", "“": "&ldquo;", "”": "&rdquo;", "„": "&bdquo;", "†": "&dagger;",
	"‡": "&Dagger;", "•": "&bull;", "…": "&hellip;", "‰": "&permil;", "′": "&prime;",
	"″": "&Prime;", "‹": "&lsaquo;", "›": "&rsaquo;", "‾": "&oline;", "⁄": "&frasl;",
	"€": "&euro;", "ℑ": "&image;", "℘": "&weierp;", "ℜ": "&real;", "™": "&trade;",
	"ℵ": "&alefsym;", "←": "&larr;", "↑": "&uarr;", "→": "&rarr;", "↓": "&darr;",
	"↔": "&harr;", "↵": "&crarr;", "⇐": "&lArr;", "⇑": "&uArr;", "⇒": "&rArr;",
	"⇓": "&dArr;", "⇔": "&hArr;", "∀": "&forall;", "∂": "&part;", "∃": "&exist;",
	"∅": "&empty;", "∇": "&nabla;", "∈": "&isin;", "∉": "&notin;", "∋": "&ni;",
	"∏": "&prod;", "∑": "&sum;", "−": "&minus;", "∗": "&lowast;", "√": "&radic;",
	"∝": "&prop;", "∞": "&infin;", "∠": "&ang;", "∧": "&and;", "∨": "&or;",
	"∩": "&cap;", "∪": "&cup;", "∫": "&int;", "∴": "&there4;", "∼": "&sim;",
	"≅": "&cong;", "≈": "&asymp;", "≠": "&ne;", "≡": "&equiv;", "≤": "&le;",
	"≥": "&ge;", "⊂": "&sub;", "⊃": "&sup;", "⊄": "&nsub;", "⊆": "&sube;",
	"⊇": "&supe;", "⊕": "&oplus;", "⊗": "&otimes;", "⊥": "&perp;", "⋅": "&sdot;",
	"⌈": "&lceil;", "⌉": "&rceil;", "⌊": "&lfloor;", "⌋": "&rfloor;", "〈": "&lang;",
	"〉": "&rang;", "◊": "&loz;", "♠": "&spades;", "♣": "&clubs;", "♥": "&hearts;",
	"♦": "&diams;",
}
var tableHtml5 = map[string]string{
	"\t": "&Tab;",
	"\n": "&NewLine;", "!": "&excl;", "#": "&num;", "$": "&dollar;", "%": "&percnt;",
	"(": "&lpar;", ")": "&rpar;", "*": "&ast;", "+": "&plus;", ",": "&comma;",
	".": "&period;", "/": "&sol;", ":": "&colon;", ";": "&semi;", "<": "&lt;",
	"<⃒": "&nvlt;", "=": "&equals;", "=⃥": "&bne;", ">": "&gt;", ">⃒": "&nvgt;",
	"?": "&quest;", "@": "&commat;", "[": "&lbrack;", "\\": "&bsol;", "]": "&rsqb;",
	"^": "&Hat;", "_": "&lowbar;", "`": "&grave;", "fj": "&fjlig;", "{": "&lbrace;",
	"|": "&vert;", "}": "&rcub;", " ": "&nbsp;", "¡": "&iexcl;", "¢": "&cent;",
	"£": "&pound;", "¤": "&curren;", "¥": "&yen;", "¦": "&brvbar;", "§": "&sect;",
	"¨": "&DoubleDot;", "©": "&copy;", "ª": "&ordf;", "«": "&laquo;", "¬": "&not;",
	"­": "&shy;", "®": "&reg;", "¯": "&macr;", "°": "&deg;", "±": "&plusmn;",
	"²": "&sup2;", "³": "&sup3;", "´": "&DiacriticalAcute;", "µ": "&micro;", "¶": "&para;",
	"·": "&CenterDot;", "¸": "&Cedilla;", "¹": "&sup1;", "º": "&ordm;", "»": "&raquo;",
	"¼": "&frac14;", "½": "&half;", "¾": "&frac34;", "¿": "&iquest;", "À": "&Agrave;",
	"Á": "&Aacute;", "Â": "&Acirc;", "Ã": "&Atilde;", "Ä": "&Auml;", "Å": "&Aring;",
	"Æ": "&AElig;", "Ç": "&Ccedil;", "È": "&Egrave;", "É": "&Eacute;", "Ê": "&Ecirc;",
	"Ë": "&Euml;", "Ì": "&Igrave;", "Í": "&Iacute;", "Î": "&Icirc;", "Ï": "&Iuml;",
	"Ð": "&ETH;", "Ñ": "&Ntilde;", "Ò": "&Ograve;", "Ó": "&Oacute;", "Ô": "&Ocirc;",
	"Õ": "&Otilde;", "Ö": "&Ouml;", "×": "&times;", "Ø": "&Oslash;", "Ù": "&Ugrave;",
	"Ú": "&Uacute;", "Û": "&Ucirc;", "Ü": "&Uuml;", "Ý": "&Yacute;", "Þ": "&THORN;",
	"ß": "&szlig;", "à": "&agrave;", "á": "&aacute;", "â": "&acirc;", "ã": "&atilde;",
	"ä": "&auml;", "å": "&aring;", "æ": "&aelig;", "ç": "&ccedil;", "è": "&egrave;",
	"é": "&eacute;", "ê": "&ecirc;", "ë": "&euml;", "ì": "&igrave;", "í": "&iacute;",
	"î": "&icirc;", "ï": "&iuml;", "ð": "&eth;", "ñ": "&ntilde;", "ò": "&ograve;",
	"ó": "&oacute;", "ô": "&ocirc;", "õ": "&otilde;", "ö": "&ouml;", "÷": "&divide;",
	"ø": "&oslash;", "ù": "&ugrave;", "ú": "&uacute;", "û": "&ucirc;", "ü": "&uuml;",
	"ý": "&yacute;", "þ": "&thorn;", "ÿ": "&yuml;", "Ā": "&Amacr;", "ā": "&amacr;",
	"Ă": "&Abreve;", "ă": "&abreve;", "Ą": "&Aogon;", "ą": "&aogon;", "Ć": "&Cacute;",
	"ć": "&cacute;", "Ĉ": "&Ccirc;", "ĉ": "&ccirc;", "Ċ": "&Cdot;", "ċ": "&cdot;",
	"Č": "&Ccaron;", "č": "&ccaron;", "Ď": "&Dcaron;", "ď": "&dcaron;", "Đ": "&Dstrok;",
	"đ": "&dstrok;", "Ē": "&Emacr;", "ē": "&emacr;", "Ė": "&Edot;", "ė": "&edot;",
	"Ę": "&Eogon;", "ę": "&eogon;", "Ě": "&Ecaron;", "ě": "&ecaron;", "Ĝ": "&Gcirc;",
	"ĝ": "&gcirc;", "Ğ": "&Gbreve;", "ğ": "&gbreve;", "Ġ": "&Gdot;", "ġ": "&gdot;",
	"Ģ": "&Gcedil;", "Ĥ": "&Hcirc;", "ĥ": "&hcirc;", "Ħ": "&Hstrok;", "ħ": "&hstrok;",
	"Ĩ": "&Itilde;", "ĩ": "&itilde;", "Ī": "&Imacr;", "ī": "&imacr;", "Į": "&Iogon;",
	"į": "&iogon;", "İ": "&Idot;", "ı": "&inodot;", "Ĳ": "&IJlig;", "ĳ": "&ijlig;",
	"Ĵ": "&Jcirc;", "ĵ": "&jcirc;", "Ķ": "&Kcedil;", "ķ": "&kcedil;", "ĸ": "&kgreen;",
	"Ĺ": "&Lacute;", "ĺ": "&lacute;", "Ļ": "&Lcedil;", "ļ": "&lcedil;", "Ľ": "&Lcaron;",
	"ľ": "&lcaron;", "Ŀ": "&Lmidot;", "ŀ": "&lmidot;", "Ł": "&Lstrok;", "ł": "&lstrok;",
	"Ń": "&Nacute;", "ń": "&nacute;", "Ņ": "&Ncedil;", "ņ": "&ncedil;", "Ň": "&Ncaron;",
	"ň": "&ncaron;", "ŉ": "&napos;", "Ŋ": "&ENG;", "ŋ": "&eng;", "Ō": "&Omacr;",
	"ō": "&omacr;", "Ő": "&Odblac;", "ő": "&odblac;", "Œ": "&OElig;", "œ": "&oelig;",
	"Ŕ": "&Racute;", "ŕ": "&racute;", "Ŗ": "&Rcedil;", "ŗ": "&rcedil;", "Ř": "&Rcaron;",
	"ř": "&rcaron;", "Ś": "&Sacute;", "ś": "&sacute;", "Ŝ": "&Scirc;", "ŝ": "&scirc;",
	"Ş": "&Scedil;", "ş": "&scedil;", "Š": "&Scaron;", "š": "&scaron;", "Ţ": "&Tcedil;",
	"ţ": "&tcedil;", "Ť": "&Tcaron;", "ť": "&tcaron;", "Ŧ": "&Tstrok;", "ŧ": "&tstrok;",
	"Ũ": "&Utilde;", "ũ": "&utilde;", "Ū": "&Umacr;", "ū": "&umacr;", "Ŭ": "&Ubreve;",
	"ŭ": "&ubreve;", "Ů": "&Uring;", "ů": "&uring;", "Ű": "&Udblac;", "ű": "&udblac;",
	"Ų": "&Uogon;", "ų": "&uogon;", "Ŵ": "&Wcirc;", "ŵ": "&wcirc;", "Ŷ": "&Ycirc;",
	"ŷ": "&ycirc;", "Ÿ": "&Yuml;", "Ź": "&Zacute;", "ź": "&zacute;", "Ż": "&Zdot;",
	"ż": "&zdot;", "Ž": "&Zcaron;", "ž": "&zcaron;", "ƒ": "&fnof;", "Ƶ": "&imped;",
	"ǵ": "&gacute;", "ȷ": "&jmath;", "ˆ": "&circ;", "ˇ": "&Hacek;", "˘": "&Breve;",
	"˙": "&dot;", "˚": "&ring;", "˛": "&ogon;", "˜": "&DiacriticalTilde;",
	"˝": "&DiacriticalDoubleAcute;", "̑": "&DownBreve;",
	"Α": "&Alpha;", "Β": "&Beta;", "Γ": "&Gamma;", "Δ": "&Delta;", "Ε": "&Epsilon;",
	"Ζ": "&Zeta;", "Η": "&Eta;", "Θ": "&Theta;", "Ι": "&Iota;", "Κ": "&Kappa;",
	"Λ": "&Lambda;", "Μ": "&Mu;", "Ν": "&Nu;", "Ξ": "&Xi;", "Ο": "&Omicron;",
	"Π": "&Pi;", "Ρ": "&Rho;", "Σ": "&Sigma;", "Τ": "&Tau;", "Υ": "&Upsilon;",
	"Φ": "&Phi;", "Χ": "&Chi;", "Ψ": "&Psi;", "Ω": "&Omega;", "α": "&alpha;",
	"β": "&beta;", "γ": "&gamma;", "δ": "&delta;", "ε": "&epsi;", "ζ": "&zeta;",
	"η": "&eta;", "θ": "&theta;", "ι": "&iota;", "κ": "&kappa;", "λ": "&lambda;",
	"μ": "&mu;", "ν": "&nu;", "ξ": "&xi;", "ο": "&omicron;", "π": "&pi;",
	"ρ": "&rho;", "ς": "&sigmav;", "σ": "&sigma;", "τ": "&tau;", "υ": "&upsi;",
	"φ": "&phi;", "χ": "&chi;", "ψ": "&psi;", "ω": "&omega;", "ϑ": "&thetasym;",
	"ϒ": "&upsih;", "ϕ": "&straightphi;", "ϖ": "&piv;", "Ϝ": "&Gammad;", "ϝ": "&gammad;",
	"ϰ": "&varkappa;", "ϱ": "&rhov;", "ϵ": "&straightepsilon;", "϶": "&backepsilon;", "Ё": "&IOcy;",
	"Ђ": "&DJcy;", "Ѓ": "&GJcy;", "Є": "&Jukcy;", "Ѕ": "&DScy;", "І": "&Iukcy;",
	"Ї": "&YIcy;", "Ј": "&Jsercy;", "Љ": "&LJcy;", "Њ": "&NJcy;", "Ћ": "&TSHcy;",
	"Ќ": "&KJcy;", "Ў": "&Ubrcy;", "Џ": "&DZcy;", "А": "&Acy;", "Б": "&Bcy;",
	"В": "&Vcy;", "Г": "&Gcy;", "Д": "&Dcy;", "Е": "&IEcy;", "Ж": "&ZHcy;",
	"З": "&Zcy;", "И": "&Icy;", "Й": "&Jcy;", "К": "&Kcy;", "Л": "&Lcy;",
	"М": "&Mcy;", "Н": "&Ncy;", "О": "&Ocy;", "П": "&Pcy;", "Р": "&Rcy;",
	"С": "&Scy;", "Т": "&Tcy;", "У": "&Ucy;", "Ф": "&Fcy;", "Х": "&KHcy;",
	"Ц": "&TScy;", "Ч": "&CHcy;", "Ш": "&SHcy;", "Щ": "&SHCHcy;", "Ъ": "&HARDcy;",
	"Ы": "&Ycy;", "Ь": "&SOFTcy;", "Э": "&Ecy;", "Ю": "&YUcy;", "Я": "&YAcy;",
	"а": "&acy;", "б": "&bcy;", "в": "&vcy;", "г": "&gcy;", "д": "&dcy;",
	"е": "&iecy;", "ж": "&zhcy;", "з": "&zcy;", "и": "&icy;", "й": "&jcy;",
	"к": "&kcy;", "л": "&lcy;", "м": "&mcy;", "н": "&ncy;", "о": "&ocy;",
	"п": "&pcy;", "р": "&rcy;", "с": "&scy;", "т": "&tcy;", "у": "&ucy;",
	"ф": "&fcy;", "х": "&khcy;", "ц": "&tscy;", "ч": "&chcy;", "ш": "&shcy;",
	"щ": "&shchcy;", "ъ": "&hardcy;", "ы": "&ycy;", "ь": "&softcy;", "э": "&ecy;",
	"ю": "&yucy;", "я": "&yacy;", "ё": "&iocy;", "ђ": "&djcy;", "ѓ": "&gjcy;",
	"є": "&jukcy;", "ѕ": "&dscy;", "і": "&iukcy;", "ї": "&yicy;", "ј": "&jsercy;",
	"љ": "&ljcy;", "њ": "&njcy;", "ћ": "&tshcy;", "ќ": "&kjcy;", "ў": "&ubrcy;",
	"џ": "&dzcy;", " ": "&ensp;", " ": "&emsp;", " ": "&emsp13;", " ": "&emsp14;",
	" ": "&numsp;", " ": "&puncsp;", " ": "&ThinSpace;", " ": "&hairsp;",
	"​": "&ZeroWidthSpace;", "‌": "&zwnj;", "‍": "&zwj;", "‎": "&lrm;", "‏": "&rlm;",
	"‐": "&hyphen;", "–": "&ndash;", "—": "&mdash;", "―": "&horbar;", "‖": "&Verbar;",
	"‘": "&OpenCurlyQuote;", "’": "&rsquo;", "‚": "&sbquo;", "“": "&OpenCurlyDoubleQuote;",
	"”": "&rdquo;", "„": "&bdquo;", "†": "&dagger;", "‡": "&Dagger;", "•": "&bull;",
	"‥": "&nldr;", "…": "&hellip;", "‰": "&permil;", "‱": "&pertenk;", "′": "&prime;",
	"″": "&Prime;", "‴": "&tprime;", "‵": "&backprime;", "‹": "&lsaquo;", "›": "&rsaquo;",
	"‾": "&oline;", "⁁": "&caret;", "⁃": "&hybull;", "⁄": "&frasl;", "⁏": "&bsemi;",
	"⁗": "&qprime;", " ": "&MediumSpace;", "  ": "&ThickSpace;", "⁠": "&NoBreak;", "⁡": "&af;",
	"⁢": "&InvisibleTimes;", "⁣": "&ic;", "€": "&euro;", "⃛": "&TripleDot;", "⃜": "&DotDot;",
	"ℂ": "&complexes;", "℅": "&incare;", "ℊ": "&gscr;", "ℋ": "&HilbertSpace;", "ℌ": "&Hfr;",
	"ℍ": "&Hopf;", "ℎ": "&planckh;", "ℏ": "&planck;", "ℐ": "&imagline;", "ℑ": "&Ifr;",
	"ℒ": "&lagran;", "ℓ": "&ell;", "ℕ": "&naturals;", "№": "&numero;", "℗": "&copysr;",
	"℘": "&wp;", "ℙ": "&primes;", "ℚ": "&rationals;", "ℛ": "&realine;", "ℜ": "&Rfr;",
	"ℝ": "&Ropf;", "℞": "&rx;", "™": "&trade;", "ℤ": "&Zopf;", "℧": "&mho;",
	"ℨ": "&Zfr;", "℩": "&iiota;", "ℬ": "&Bscr;", "ℭ": "&Cfr;", "ℯ": "&escr;",
	"ℰ": "&expectation;", "ℱ": "&Fouriertrf;", "ℳ": "&Mellintrf;", "ℴ": "&orderof;",
	"ℵ": "&aleph;", "ℶ": "&beth;", "ℷ": "&gimel;", "ℸ": "&daleth;",
	"ⅅ": "&CapitalDifferentialD;", "ⅆ": "&DifferentialD;", "ⅇ": "&exponentiale;",
	"ⅈ": "&ImaginaryI;", "⅓": "&frac13;", "⅔": "&frac23;", "⅕": "&frac15;",
	"⅖": "&frac25;", "⅗": "&frac35;", "⅘": "&frac45;", "⅙": "&frac16;", "⅚": "&frac56;",
	"⅛": "&frac18;", "⅜": "&frac38;", "⅝": "&frac58;", "⅞": "&frac78;", "←": "&larr;",
	"↑": "&uarr;", "→": "&srarr;", "↓": "&darr;", "↔": "&harr;", "↕": "&UpDownArrow;",
	"↖": "&nwarrow;", "↗": "&UpperRightArrow;", "↘": "&LowerRightArrow;",
	"↙": "&swarr;", "↚": "&nleftarrow;", "↛": "&nrarr;", "↝": "&rarrw;",
	"↝̸": "&nrarrw;", "↞": "&Larr;", "↟": "&Uarr;", "↠": "&twoheadrightarrow;",
	"↡": "&Darr;", "↢": "&larrtl;", "↣": "&rarrtl;", "↤": "&LeftTeeArrow;",
	"↥": "&UpTeeArrow;", "↦": "&map;", "↧": "&DownTeeArrow;", "↩": "&larrhk;",
	"↪": "&rarrhk;", "↫": "&larrlp;", "↬": "&looparrowright;", "↭": "&harrw;",
	"↮": "&nleftrightarrow;", "↰": "&Lsh;", "↱": "&rsh;", "↲": "&ldsh;",
	"↳": "&rdsh;", "↵": "&crarr;", "↶": "&curvearrowleft;",
	"↷": "&curarr;", "↺": "&olarr;", "↻": "&orarr;", "↼": "&leftharpoonup;",
	"↽": "&leftharpoondown;", "↾": "&RightUpVector;", "↿": "&uharl;", "⇀": "&rharu;",
	"⇁": "&rhard;", "⇂": "&RightDownVector;", "⇃": "&dharl;",
	"⇄": "&rightleftarrows;", "⇅": "&udarr;", "⇆": "&lrarr;", "⇇": "&llarr;", "⇈": "&upuparrows;",
	"⇉": "&rrarr;", "⇊": "&downdownarrows;", "⇋": "&leftrightharpoons;", "⇌": "&rightleftharpoons;",
	"⇍": "&nLeftarrow;", "⇎": "&nhArr;", "⇏": "&nrArr;", "⇐": "&DoubleLeftArrow;",
	"⇑": "&DoubleUpArrow;", "⇒": "&Implies;", "⇓": "&Downarrow;", "⇔": "&hArr;",
	"⇕": "&Updownarrow;", "⇖": "&nwArr;", "⇗": "&neArr;", "⇘": "&seArr;",
	"⇙": "&swArr;", "⇚": "&lAarr;", "⇛": "&rAarr;", "⇝": "&zigrarr;", "⇤": "&LeftArrowBar;",
	"⇥": "&RightArrowBar;", "⇵": "&DownArrowUpArrow;", "⇽": "&loarr;", "⇾": "&roarr;",
	"⇿": "&hoarr;", "∀": "&forall;", "∁": "&comp;", "∂": "&part;", "∂̸": "&npart;",
	"∃": "&Exists;", "∄": "&nexist;", "∅": "&empty;", "∇": "&nabla;", "∈": "&isinv;",
	"∉": "&notin;", "∋": "&ReverseElement;", "∌": "&notniva;", "∏": "&prod;", "∐": "&Coproduct;",
	"∑": "&sum;", "−": "&minus;", "∓": "&MinusPlus;", "∔": "&plusdo;", "∖": "&ssetmn;",
	"∗": "&lowast;", "∘": "&compfn;", "√": "&Sqrt;", "∝": "&prop;", "∞": "&infin;",
	"∟": "&angrt;", "∠": "&angle;", "∠⃒": "&nang;", "∡": "&angmsd;", "∢": "&angsph;",
	"∣": "&mid;", "∤": "&nshortmid;", "∥": "&shortparallel;", "∦": "&nparallel;", "∧": "&and;",
	"∨": "&or;", "∩": "&cap;", "∩︀": "&caps;", "∪": "&cup;", "∪︀": "&cups;",
	"∫": "&Integral;", "∬": "&Int;", "∭": "&tint;", "∮": "&ContourIntegral;",
	"∯": "&DoubleContourIntegral;",
	"∰": "&Cconint;", "∱": "&cwint;", "∲": "&cwconint;", "∳": "&awconint;", "∴": "&there4;",
	"∵": "&Because;", "∶": "&ratio;", "∷": "&Colon;", "∸": "&minusd;", "∺": "&mDDot;",
	"∻": "&homtht;", "∼": "&sim;", "∼⃒": "&nvsim;", "∽": "&bsim;", "∽̱": "&race;",
	"∾": "&ac;", "∾̳": "&acE;", "∿": "&acd;", "≀": "&wr;", "≁": "&NotTilde;",
	"≂": "&esim;", "≂̸": "&nesim;", "≃": "&simeq;", "≄": "&nsime;", "≅": "&TildeFullEqual;",
	"≆": "&simne;", "≇": "&ncong;", "≈": "&approx;", "≉": "&napprox;", "≊": "&ape;",
	"≋": "&apid;", "≋̸": "&napid;", "≌": "&bcong;", "≍": "&CupCap;", "≍⃒": "&nvap;",
	"≎": "&bump;", "≎̸": "&nbump;", "≏": "&HumpEqual;", "≏̸": "&nbumpe;", "≐": "&esdot;",
	"≐̸": "&nedot;", "≑": "&doteqdot;", "≒": "&fallingdotseq;", "≓": "&risingdotseq;", "≔": "&coloneq;",
	"≕": "&eqcolon;", "≖": "&ecir;", "≗": "&circeq;", "≙": "&wedgeq;", "≚": "&veeeq;",
	"≜": "&triangleq;", "≟": "&equest;", "≠": "&NotEqual;", "≡": "&Congruent;", "≡⃥": "&bnequiv;",
	"≢": "&NotCongruent;", "≤": "&leq;", "≤⃒": "&nvle;", "≥": "&ge;", "≥⃒": "&nvge;",
	"≦": "&lE;", "≦̸": "&nlE;", "≧": "&geqq;", "≧̸": "&NotGreaterFullEqual;", "≨": "&lneqq;",
	"≨︀": "&lvertneqq;", "≩": "&gneqq;", "≩︀": "&gvertneqq;", "≪": "&ll;", "≪̸": "&nLtv;",
	"≪⃒": "&nLt;", "≫": "&gg;", "≫̸": "&NotGreaterGreater;", "≫⃒": "&nGt;", "≬": "&between;",
	"≭": "&NotCupCap;", "≮": "&NotLess;", "≯": "&ngtr;", "≰": "&NotLessEqual;", "≱": "&ngeq;",
	"≲": "&LessTilde;", "≳": "&GreaterTilde;", "≴": "&nlsim;", "≵": "&ngsim;", "≶": "&lessgtr;",
	"≷": "&gl;", "≸": "&ntlg;", "≹": "&NotGreaterLess;", "≺": "&prec;", "≻": "&succ;",
	"≼": "&PrecedesSlantEqual;", "≽": "&succcurlyeq;", "≾": "&precsim;", "≿": "&SucceedsTilde;",
	"≿̸": "&NotSucceedsTilde;",
	"⊀":  "&npr;", "⊁": "&NotSucceeds;", "⊂": "&sub;", "⊂⃒": "&vnsub;", "⊃": "&sup;",
	"⊃⃒": "&nsupset;", "⊄": "&nsub;", "⊅": "&nsup;", "⊆": "&SubsetEqual;", "⊇": "&supe;",
	"⊈": "&NotSubsetEqual;", "⊉": "&NotSupersetEqual;", "⊊": "&subsetneq;",
	"⊊︀": "&vsubne;", "⊋": "&supsetneq;",
	"⊋︀": "&vsupne;", "⊍": "&cupdot;", "⊎": "&UnionPlus;", "⊏": "&sqsub;", "⊏̸": "&NotSquareSubset;",
	"⊐": "&sqsupset;", "⊐̸": "&NotSquareSuperset;", "⊑": "&SquareSubsetEqual;",
	"⊒": "&SquareSupersetEqual;", "⊓": "&sqcap;",
	"⊓︀": "&sqcaps;", "⊔": "&sqcup;", "⊔︀": "&sqcups;", "⊕": "&CirclePlus;", "⊖": "&ominus;",
	"⊗": "&CircleTimes;", "⊘": "&osol;", "⊙": "&CircleDot;", "⊚": "&ocir;", "⊛": "&oast;",
	"⊝": "&odash;", "⊞": "&boxplus;", "⊟": "&boxminus;", "⊠": "&timesb;", "⊡": "&sdotb;",
	"⊢": "&vdash;", "⊣": "&dashv;", "⊤": "&DownTee;", "⊥": "&perp;", "⊧": "&models;",
	"⊨": "&DoubleRightTee;", "⊩": "&Vdash;", "⊪": "&Vvdash;", "⊫": "&VDash;", "⊬": "&nvdash;",
	"⊭": "&nvDash;", "⊮": "&nVdash;", "⊯": "&nVDash;", "⊰": "&prurel;", "⊲": "&vartriangleleft;",
	"⊳": "&vrtri;", "⊴": "&LeftTriangleEqual;", "⊴⃒": "&nvltrie;",
	"⊵": "&RightTriangleEqual;", "⊵⃒": "&nvrtrie;",
	"⊶": "&origof;", "⊷": "&imof;", "⊸": "&mumap;", "⊹": "&hercon;", "⊺": "&intcal;",
	"⊻": "&veebar;", "⊽": "&barvee;", "⊾": "&angrtvb;", "⊿": "&lrtri;", "⋀": "&xwedge;",
	"⋁": "&xvee;", "⋂": "&bigcap;", "⋃": "&bigcup;", "⋄": "&diamond;", "⋅": "&sdot;",
	"⋆": "&Star;", "⋇": "&divonx;", "⋈": "&bowtie;", "⋉": "&ltimes;", "⋊": "&rtimes;",
	"⋋": "&lthree;", "⋌": "&rthree;", "⋍": "&backsimeq;", "⋎": "&curlyvee;", "⋏": "&curlywedge;",
	"⋐": "&Sub;", "⋑": "&Supset;", "⋒": "&Cap;", "⋓": "&Cup;", "⋔": "&pitchfork;",
	"⋕": "&epar;", "⋖": "&lessdot;", "⋗": "&gtrdot;", "⋘": "&Ll;", "⋘̸": "&nLl;",
	"⋙": "&Gg;", "⋙̸": "&nGg;", "⋚": "&lesseqgtr;", "⋚︀": "&lesg;", "⋛": "&gtreqless;",
	"⋛︀": "&gesl;", "⋞": "&curlyeqprec;", "⋟": "&cuesc;",
	"⋠": "&NotPrecedesSlantEqual;", "⋡": "&NotSucceedsSlantEqual;",
	"⋢": "&NotSquareSubsetEqual;", "⋣": "&NotSquareSupersetEqual;",
	"⋦": "&lnsim;", "⋧": "&gnsim;", "⋨": "&precnsim;",
	"⋩": "&scnsim;", "⋪": "&nltri;", "⋫": "&ntriangleright;",
	"⋬": "&nltrie;", "⋭": "&NotRightTriangleEqual;",
	"⋮": "&vellip;", "⋯": "&ctdot;", "⋰": "&utdot;", "⋱": "&dtdot;", "⋲": "&disin;",
	"⋳": "&isinsv;", "⋴": "&isins;", "⋵": "&isindot;", "⋵̸": "&notindot;", "⋶": "&notinvc;",
	"⋷": "&notinvb;", "⋹": "&isinE;", "⋹̸": "&notinE;", "⋺": "&nisd;", "⋻": "&xnis;",
	"⋼": "&nis;", "⋽": "&notnivc;", "⋾": "&notnivb;", "⌅": "&barwed;", "⌆": "&doublebarwedge;",
	"⌈": "&lceil;", "⌉": "&RightCeiling;", "⌊": "&LeftFloor;", "⌋": "&RightFloor;", "⌌": "&drcrop;",
	"⌍": "&dlcrop;", "⌎": "&urcrop;", "⌏": "&ulcrop;", "⌐": "&bnot;", "⌒": "&profline;",
	"⌓": "&profsurf;", "⌕": "&telrec;", "⌖": "&target;", "⌜": "&ulcorner;", "⌝": "&urcorner;",
	"⌞": "&llcorner;", "⌟": "&drcorn;", "⌢": "&frown;", "⌣": "&smile;", "⌭": "&cylcty;",
	"⌮": "&profalar;", "⌶": "&topbot;", "⌽": "&ovbar;", "⌿": "&solbar;", "⍼": "&angzarr;",
	"⎰": "&lmoust;", "⎱": "&rmoust;", "⎴": "&OverBracket;", "⎵": "&bbrk;", "⎶": "&bbrktbrk;",
	"⏜": "&OverParenthesis;", "⏝": "&UnderParenthesis;",
	"⏞": "&OverBrace;", "⏟": "&UnderBrace;", "⏢": "&trpezium;",
	"⏧": "&elinters;", "␣": "&blank;", "Ⓢ": "&oS;", "─": "&HorizontalLine;", "│": "&boxv;",
	"┌": "&boxdr;", "┐": "&boxdl;", "└": "&boxur;", "┘": "&boxul;", "├": "&boxvr;",
	"┤": "&boxvl;", "┬": "&boxhd;", "┴": "&boxhu;", "┼": "&boxvh;", "═": "&boxH;",
	"║": "&boxV;", "╒": "&boxdR;", "╓": "&boxDr;", "╔": "&boxDR;", "╕": "&boxdL;",
	"╖": "&boxDl;", "╗": "&boxDL;", "╘": "&boxuR;", "╙": "&boxUr;", "╚": "&boxUR;",
	"╛": "&boxuL;", "╜": "&boxUl;", "╝": "&boxUL;", "╞": "&boxvR;", "╟": "&boxVr;",
	"╠": "&boxVR;", "╡": "&boxvL;", "╢": "&boxVl;", "╣": "&boxVL;", "╤": "&boxHd;",
	"╥": "&boxhD;", "╦": "&boxHD;", "╧": "&boxHu;", "╨": "&boxhU;", "╩": "&boxHU;",
	"╪": "&boxvH;", "╫": "&boxVh;", "╬": "&boxVH;", "▀": "&uhblk;", "▄": "&lhblk;",
	"█": "&block;", "░": "&blk14;", "▒": "&blk12;", "▓": "&blk34;", "□": "&Square;",
	"▪": "&squarf;", "▫": "&EmptyVerySmallSquare;", "▭": "&rect;", "▮": "&marker;", "▱": "&fltns;",
	"△": "&bigtriangleup;", "▴": "&blacktriangle;", "▵": "&triangle;",
	"▸": "&blacktriangleright;", "▹": "&rtri;",
	"▽": "&bigtriangledown;", "▾": "&blacktriangledown;", "▿": "&triangledown;",
	"◂": "&blacktriangleleft;", "◃": "&ltri;",
	"◊": "&lozenge;", "○": "&cir;", "◬": "&tridot;", "◯": "&bigcirc;", "◸": "&ultri;",
	"◹": "&urtri;", "◺": "&lltri;", "◻": "&EmptySmallSquare;",
	"◼": "&FilledSmallSquare;", "★": "&starf;",
	"☆": "&star;", "☎": "&phone;", "♀": "&female;", "♂": "&male;", "♠": "&spadesuit;",
	"♣": "&clubs;", "♥": "&hearts;", "♦": "&diamondsuit;", "♪": "&sung;", "♭": "&flat;",
	"♮": "&natur;", "♯": "&sharp;", "✓": "&check;", "✗": "&cross;", "✠": "&maltese;",
	"✶": "&sext;", "❘": "&VerticalSeparator;", "❲": "&lbbrk;", "❳": "&rbbrk;", "⟈": "&bsolhsub;",
	"⟉": "&suphsol;", "⟦": "&LeftDoubleBracket;", "⟧": "&RightDoubleBracket;",
	"⟨": "&langle;", "⟩": "&RightAngleBracket;",
	"⟪": "&Lang;", "⟫": "&Rang;", "⟬": "&loang;", "⟭": "&roang;", "⟵": "&longleftarrow;",
	"⟶": "&LongRightArrow;", "⟷": "&LongLeftRightArrow;", "⟸": "&xlArr;",
	"⟹": "&DoubleLongRightArrow;", "⟺": "&xhArr;",
	"⟼": "&xmap;", "⟿": "&dzigrarr;", "⤂": "&nvlArr;", "⤃": "&nvrArr;", "⤄": "&nvHarr;",
	"⤅": "&Map;", "⤌": "&lbarr;", "⤍": "&bkarow;", "⤎": "&lBarr;", "⤏": "&dbkarow;",
	"⤐": "&drbkarow;", "⤑": "&DDotrahd;", "⤒": "&UpArrowBar;", "⤓": "&DownArrowBar;", "⤖": "&Rarrtl;",
	"⤙": "&latail;", "⤚": "&ratail;", "⤛": "&lAtail;", "⤜": "&rAtail;", "⤝": "&larrfs;",
	"⤞": "&rarrfs;", "⤟": "&larrbfs;", "⤠": "&rarrbfs;", "⤣": "&nwarhk;", "⤤": "&nearhk;",
	"⤥": "&searhk;", "⤦": "&swarhk;", "⤧": "&nwnear;", "⤨": "&toea;", "⤩": "&seswar;",
	"⤪": "&swnwar;", "⤳": "&rarrc;", "⤳̸": "&nrarrc;", "⤵": "&cudarrr;", "⤶": "&ldca;",
	"⤷": "&rdca;", "⤸": "&cudarrl;", "⤹": "&larrpl;", "⤼": "&curarrm;", "⤽": "&cularrp;",
	"⥅": "&rarrpl;", "⥈": "&harrcir;", "⥉": "&Uarrocir;", "⥊": "&lurdshar;", "⥋": "&ldrushar;",
	"⥎": "&LeftRightVector;", "⥏": "&RightUpDownVector;", "⥐": "&DownLeftRightVector;",
	"⥑": "&LeftUpDownVector;", "⥒": "&LeftVectorBar;",
	"⥓": "&RightVectorBar;", "⥔": "&RightUpVectorBar;", "⥕": "&RightDownVectorBar;",
	"⥖": "&DownLeftVectorBar;", "⥗": "&DownRightVectorBar;",
	"⥘": "&LeftUpVectorBar;", "⥙": "&LeftDownVectorBar;", "⥚": "&LeftTeeVector;",
	"⥛": "&RightTeeVector;", "⥜": "&RightUpTeeVector;",
	"⥝": "&RightDownTeeVector;", "⥞": "&DownLeftTeeVector;", "⥟": "&DownRightTeeVector;",
	"⥠": "&LeftUpTeeVector;", "⥡": "&LeftDownTeeVector;",
	"⥢": "&lHar;", "⥣": "&uHar;", "⥤": "&rHar;", "⥥": "&dHar;", "⥦": "&luruhar;",
	"⥧": "&ldrdhar;", "⥨": "&ruluhar;", "⥩": "&rdldhar;", "⥪": "&lharul;", "⥫": "&llhard;",
	"⥬": "&rharul;", "⥭": "&lrhard;", "⥮": "&udhar;", "⥯": "&ReverseUpEquilibrium;", "⥰": "&RoundImplies;",
	"⥱": "&erarr;", "⥲": "&simrarr;", "⥳": "&larrsim;", "⥴": "&rarrsim;", "⥵": "&rarrap;",
	"⥶": "&ltlarr;", "⥸": "&gtrarr;", "⥹": "&subrarr;", "⥻": "&suplarr;", "⥼": "&lfisht;",
	"⥽": "&rfisht;", "⥾": "&ufisht;", "⥿": "&dfisht;", "⦅": "&lopar;", "⦆": "&ropar;",
	"⦋": "&lbrke;", "⦌": "&rbrke;", "⦍": "&lbrkslu;", "⦎": "&rbrksld;", "⦏": "&lbrksld;",
	"⦐": "&rbrkslu;", "⦑": "&langd;", "⦒": "&rangd;", "⦓": "&lparlt;", "⦔": "&rpargt;",
	"⦕": "&gtlPar;", "⦖": "&ltrPar;", "⦚": "&vzigzag;", "⦜": "&vangrt;", "⦝": "&angrtvbd;",
	"⦤": "&ange;", "⦥": "&range;", "⦦": "&dwangle;", "⦧": "&uwangle;", "⦨": "&angmsdaa;",
	"⦩": "&angmsdab;", "⦪": "&angmsdac;", "⦫": "&angmsdad;", "⦬": "&angmsdae;", "⦭": "&angmsdaf;",
	"⦮": "&angmsdag;", "⦯": "&angmsdah;", "⦰": "&bemptyv;", "⦱": "&demptyv;", "⦲": "&cemptyv;",
	"⦳": "&raemptyv;", "⦴": "&laemptyv;", "⦵": "&ohbar;", "⦶": "&omid;", "⦷": "&opar;",
	"⦹": "&operp;", "⦻": "&olcross;", "⦼": "&odsold;", "⦾": "&olcir;", "⦿": "&ofcir;",
	"⧀": "&olt;", "⧁": "&ogt;", "⧂": "&cirscir;", "⧃": "&cirE;", "⧄": "&solb;",
	"⧅": "&bsolb;", "⧉": "&boxbox;", "⧍": "&trisb;", "⧎": "&rtriltri;", "⧏": "&LeftTriangleBar;",
	"⧏̸": "&NotLeftTriangleBar;", "⧐": "&RightTriangleBar;", "⧐̸": "&NotRightTriangleBar;",
	"⧜": "&iinfin;", "⧝": "&infintie;",
	"⧞": "&nvinfin;", "⧣": "&eparsl;", "⧤": "&smeparsl;", "⧥": "&eqvparsl;", "⧫": "&lozf;",
	"⧴": "&RuleDelayed;", "⧶": "&dsol;", "⨀": "&xodot;", "⨁": "&bigoplus;", "⨂": "&bigotimes;",
	"⨄": "&biguplus;", "⨆": "&bigsqcup;", "⨌": "&iiiint;", "⨍": "&fpartint;", "⨐": "&cirfnint;",
	"⨑": "&awint;", "⨒": "&rppolint;", "⨓": "&scpolint;", "⨔": "&npolint;", "⨕": "&pointint;",
	"⨖": "&quatint;", "⨗": "&intlarhk;", "⨢": "&pluscir;", "⨣": "&plusacir;", "⨤": "&simplus;",
	"⨥": "&plusdu;", "⨦": "&plussim;", "⨧": "&plustwo;", "⨩": "&mcomma;", "⨪": "&minusdu;",
	"⨭": "&loplus;", "⨮": "&roplus;", "⨯": "&Cross;", "⨰": "&timesd;", "⨱": "&timesbar;",
	"⨳": "&smashp;", "⨴": "&lotimes;", "⨵": "&rotimes;", "⨶": "&otimesas;", "⨷": "&Otimes;",
	"⨸": "&odiv;", "⨹": "&triplus;", "⨺": "&triminus;", "⨻": "&tritime;", "⨼": "&iprod;",
	"⨿": "&amalg;", "⩀": "&capdot;", "⩂": "&ncup;", "⩃": "&ncap;", "⩄": "&capand;",
	"⩅": "&cupor;", "⩆": "&cupcap;", "⩇": "&capcup;", "⩈": "&cupbrcap;", "⩉": "&capbrcup;",
	"⩊": "&cupcup;", "⩋": "&capcap;", "⩌": "&ccups;", "⩍": "&ccaps;", "⩐": "&ccupssm;",
	"⩓": "&And;", "⩔": "&Or;", "⩕": "&andand;", "⩖": "&oror;", "⩗": "&orslope;",
	"⩘": "&andslope;", "⩚": "&andv;", "⩛": "&orv;", "⩜": "&andd;", "⩝": "&ord;",
	"⩟": "&wedbar;", "⩦": "&sdote;", "⩪": "&simdot;", "⩭": "&congdot;", "⩭̸": "&ncongdot;",
	"⩮": "&easter;", "⩯": "&apacir;", "⩰": "&apE;", "⩰̸": "&napE;", "⩱": "&eplus;",
	"⩲": "&pluse;", "⩳": "&Esim;", "⩴": "&Colone;", "⩵": "&Equal;", "⩷": "&ddotseq;",
	"⩸": "&equivDD;", "⩹": "&ltcir;", "⩺": "&gtcir;", "⩻": "&ltquest;", "⩼": "&gtquest;",
	"⩽": "&les;", "⩽̸": "&nles;", "⩾": "&ges;", "⩾̸": "&nges;", "⩿": "&lesdot;",
	"⪀": "&gesdot;", "⪁": "&lesdoto;", "⪂": "&gesdoto;", "⪃": "&lesdotor;", "⪄": "&gesdotol;",
	"⪅": "&lap;", "⪆": "&gap;", "⪇": "&lne;", "⪈": "&gne;", "⪉": "&lnap;",
	"⪊": "&gnap;", "⪋": "&lesseqqgtr;", "⪌": "&gEl;", "⪍": "&lsime;", "⪎": "&gsime;",
	"⪏": "&lsimg;", "⪐": "&gsiml;", "⪑": "&lgE;", "⪒": "&glE;", "⪓": "&lesges;",
	"⪔": "&gesles;", "⪕": "&els;", "⪖": "&egs;", "⪗": "&elsdot;", "⪘": "&egsdot;",
	"⪙": "&el;", "⪚": "&eg;", "⪝": "&siml;", "⪞": "&simg;", "⪟": "&simlE;",
	"⪠": "&simgE;", "⪡": "&LessLess;", "⪡̸": "&NotNestedLessLess;",
	"⪢": "&GreaterGreater;", "⪢̸": "&NotNestedGreaterGreater;",
	"⪤": "&glj;", "⪥": "&gla;", "⪦": "&ltcc;", "⪧": "&gtcc;", "⪨": "&lescc;",
	"⪩": "&gescc;", "⪪": "&smt;", "⪫": "&lat;", "⪬": "&smte;", "⪬︀": "&smtes;",
	"⪭": "&late;", "⪭︀": "&lates;", "⪮": "&bumpE;", "⪯": "&preceq;", "⪯̸": "&NotPrecedesEqual;",
	"⪰": "&SucceedsEqual;", "⪰̸": "&NotSucceedsEqual;", "⪳": "&prE;", "⪴": "&scE;", "⪵": "&precneqq;",
	"⪶": "&scnE;", "⪷": "&precapprox;", "⪸": "&succapprox;", "⪹": "&precnapprox;", "⪺": "&succnapprox;",
	"⪻": "&Pr;", "⪼": "&Sc;", "⪽": "&subdot;", "⪾": "&supdot;", "⪿": "&subplus;",
	"⫀": "&supplus;", "⫁": "&submult;", "⫂": "&supmult;", "⫃": "&subedot;", "⫄": "&supedot;",
	"⫅": "&subE;", "⫅̸": "&nsubE;", "⫆": "&supseteqq;", "⫆̸": "&nsupseteqq;", "⫇": "&subsim;",
	"⫈": "&supsim;", "⫋": "&subsetneqq;", "⫋︀": "&vsubnE;", "⫌": "&supnE;", "⫌︀": "&varsupsetneqq;",
	"⫏": "&csub;", "⫐": "&csup;", "⫑": "&csube;", "⫒": "&csupe;", "⫓": "&subsup;",
	"⫔": "&supsub;", "⫕": "&subsub;", "⫖": "&supsup;", "⫗": "&suphsub;", "⫘": "&supdsub;",
	"⫙": "&forkv;", "⫚": "&topfork;", "⫛": "&mlcp;", "⫤": "&Dashv;", "⫦": "&Vdashl;",
	"⫧": "&Barv;", "⫨": "&vBar;", "⫩": "&vBarv;", "⫫": "&Vbar;", "⫬": "&Not;",
	"⫭": "&bNot;", "⫮": "&rnmid;", "⫯": "&cirmid;", "⫰": "&midcir;", "⫱": "&topcir;",
	"⫲": "&nhpar;", "⫳": "&parsim;", "⫽": "&parsl;", "⫽⃥": "&nparsl;", "ﬀ": "&fflig;",
	"ﬁ": "&filig;", "ﬂ": "&fllig;", "ﬃ": "&ffilig;", "ﬄ": "&ffllig;", "𝒜": "&Ascr;",
	"𝒞": "&Cscr;", "𝒟": "&Dscr;", "𝒢": "&Gscr;", "𝒥": "&Jscr;", "𝒦": "&Kscr;",
	"𝒩": "&Nscr;", "𝒪": "&Oscr;", "𝒫": "&Pscr;", "𝒬": "&Qscr;", "𝒮": "&Sscr;",
	"𝒯": "&Tscr;", "𝒰": "&Uscr;", "𝒱": "&Vscr;", "𝒲": "&Wscr;", "𝒳": "&Xscr;",
	"𝒴": "&Yscr;", "𝒵": "&Zscr;", "𝒶": "&ascr;", "𝒷": "&bscr;", "𝒸": "&cscr;",
	"𝒹": "&dscr;", "𝒻": "&fscr;", "𝒽": "&hscr;", "𝒾": "&iscr;", "𝒿": "&jscr;",
	"𝓀": "&kscr;", "𝓁": "&lscr;", "𝓂": "&mscr;", "𝓃": "&nscr;", "𝓅": "&pscr;",
	"𝓆": "&qscr;", "𝓇": "&rscr;", "𝓈": "&sscr;", "𝓉": "&tscr;", "𝓊": "&uscr;",
	"𝓋": "&vscr;", "𝓌": "&wscr;", "𝓍": "&xscr;", "𝓎": "&yscr;", "𝓏": "&zscr;",
	"𝔄": "&Afr;", "𝔅": "&Bfr;", "𝔇": "&Dfr;", "𝔈": "&Efr;", "𝔉": "&Ffr;",
	"𝔊": "&Gfr;", "𝔍": "&Jfr;", "𝔎": "&Kfr;", "𝔏": "&Lfr;", "𝔐": "&Mfr;",
	"𝔑": "&Nfr;", "𝔒": "&Ofr;", "𝔓": "&Pfr;", "𝔔": "&Qfr;", "𝔖": "&Sfr;",
	"𝔗": "&Tfr;", "𝔘": "&Ufr;", "𝔙": "&Vfr;", "𝔚": "&Wfr;", "𝔛": "&Xfr;",
	"𝔜": "&Yfr;", "𝔞": "&afr;", "𝔟": "&bfr;", "𝔠": "&cfr;", "𝔡": "&dfr;",
	"𝔢": "&efr;", "𝔣": "&ffr;", "𝔤": "&gfr;", "𝔥": "&hfr;", "𝔦": "&ifr;",
	"𝔧": "&jfr;", "𝔨": "&kfr;", "𝔩": "&lfr;", "𝔪": "&mfr;", "𝔫": "&nfr;",
	"𝔬": "&ofr;", "𝔭": "&pfr;", "𝔮": "&qfr;", "𝔯": "&rfr;", "𝔰": "&sfr;",
	"𝔱": "&tfr;", "𝔲": "&ufr;", "𝔳": "&vfr;", "𝔴": "&wfr;", "𝔵": "&xfr;",
	"𝔶": "&yfr;", "𝔷": "&zfr;", "𝔸": "&Aopf;", "𝔹": "&Bopf;", "𝔻": "&Dopf;",
	"𝔼": "&Eopf;", "𝔽": "&Fopf;", "𝔾": "&Gopf;", "𝕀": "&Iopf;", "𝕁": "&Jopf;",
	"𝕂": "&Kopf;", "𝕃": "&Lopf;", "𝕄": "&Mopf;", "𝕆": "&Oopf;", "𝕊": "&Sopf;",
	"𝕋": "&Topf;", "𝕌": "&Uopf;", "𝕍": "&Vopf;", "𝕎": "&Wopf;", "𝕏": "&Xopf;",
	"𝕐": "&Yopf;", "𝕒": "&aopf;", "𝕓": "&bopf;", "𝕔": "&copf;", "𝕕": "&dopf;",
	"𝕖": "&eopf;", "𝕗": "&fopf;", "𝕘": "&gopf;", "𝕙": "&hopf;", "𝕚": "&iopf;",
	"𝕛": "&jopf;", "𝕜": "&kopf;", "𝕝": "&lopf;", "𝕞": "&mopf;", "𝕟": "&nopf;",
	"𝕠": "&oopf;", "𝕡": "&popf;", "𝕢": "&qopf;", "𝕣": "&ropf;", "𝕤": "&sopf;",
	"𝕥": "&topf;", "𝕦": "&uopf;", "𝕧": "&vopf;", "𝕨": "&wopf;", "𝕩": "&xopf;",
	"𝕪": "&yopf;", "𝕫": "&zopf;",
}

type translationEntry struct {
	key   string
	value string
}

func getHtmlTranslationTable(table phpv.ZInt, flags phpv.ZInt) []*translationEntry {
	entries := []*translationEntry{}
	quoteFlags := flags & (ENT_HTML_QUOTE_DOUBLE | ENT_HTML_QUOTE_SINGLE)
	flags &= ENT_HTML_DOC_TYPE_MASK

	if quoteFlags&ENT_HTML_QUOTE_DOUBLE > 0 {
		entries = append(entries, &translationEntry{`"`, `&quot;`})
	}

	// Note: flags is divided into groups
	// the first two bits are for the quotes:
	//   ENT_COMPAT, ENT_QUOTES, ENT_NOQUOTES
	// while the upper remaining bits are for the table:
	//   ENT_HTML401, ENT_XML1, ENT_XHTML, ENT_HTML5
	//
	// The quote flags should be zeroed out before processing
	// the table flags. This is because
	// ENT_HTML401, ENT_XML1, ENT_XHTML, ENT_HTML5
	// doesn't act like normal bit flags,
	// although they deceptively use power of 2 values.
	//
	// For instance, while ENT_HTML5 == ENT_XML1 | ENT_XHTML,
	// get_html_translation_table($x, ENT_HTML5) !=
	// 		get_html_translation_table($x, ENT_XML1 | ENT_XHTML)
	//
	// As such, table flags should be considered regular separate enum values.

	if table == HTML_SPECIALCHARS {
		if quoteFlags&ENT_HTML_QUOTE_SINGLE > 0 {
			if flags&(ENT_XML1|ENT_XHTML) > 0 {
				entries = append(entries, &translationEntry{`'`, `&apos;`})
			} else {
				entries = append(entries, &translationEntry{`'`, `&#039;`})
			}
		}

		for k, v := range tableBase {
			entries = append(entries, &translationEntry{k, v})
		}
	} else {
		if quoteFlags&ENT_HTML_QUOTE_SINGLE > 0 {
			if flags&ENT_XML1 > 0 {
				entries = append(entries, &translationEntry{`'`, `&apos;`})
			} else {
				entries = append(entries, &translationEntry{`'`, `&#039;`})
			}
		}

		for k, v := range tableBase {
			entries = append(entries, &translationEntry{k, v})
		}

		if flags != ENT_XML1 {
			if flags == ENT_HTML401 || flags == ENT_XHTML {
				for k, v := range table401 {
					entries = append(entries, &translationEntry{k, v})
				}
			} else if flags == ENT_HTML5 {
				for k, v := range tableHtml5 {
					entries = append(entries, &translationEntry{k, v})
				}
			}
		}
	}

	return entries
}

// > func array get_html_translation_table ( [ int $table = HTML_SPECIALCHARS [, int $flags = ENT_COMPAT | ENT_HTML401 [, string $encoding = "UTF-8" ]]] )
func fncGetHtmlTranslationTable(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var tableArg core.Optional[phpv.ZInt]
	var flagsArgs core.Optional[phpv.ZInt]
	var encodingArgs core.Optional[phpv.ZString]
	_, err := core.Expand(ctx, args, &tableArg, &flagsArgs, &encodingArgs)
	if err != nil {
		return nil, err
	}

	table := tableArg.GetOrDefault(HTML_SPECIALCHARS)
	flags := flagsArgs.GetOrDefault(ENT_COMPAT | ENT_HTML401)

	if encodingArgs.HasArg() && strings.ToUpper(string(encodingArgs.Get())) != "UTF-8" {
		// TODO: encoding := encodingArgs.GetOrDefault("UTF-8")
		return nil, ctx.FuncErrorf("only UTF-8 encoding is supported for now")
	}

	entries := getHtmlTranslationTable(table, flags)
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].key < entries[j].key
	})

	result := phpv.NewZArray()
	for _, row := range entries {
		result.OffsetSet(ctx, phpv.ZStr(row.key), phpv.ZStr(row.value))
	}
	return result.ZVal(), nil
}

// > func string htmlspecialchars ( string $string [, int $flags = ENT_COMPAT | ENT_HTML401 [, string $encoding = ini_get("default_charset") [, bool $double_encode = TRUE ]]] )
func fncHtmlSpecialChars(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var str phpv.ZString
	var flagsArg core.Optional[phpv.ZInt]
	var encodingArg core.Optional[phpv.ZString]
	var doubleEncodeArg core.Optional[phpv.ZBool]
	_, err := core.Expand(ctx, args, &str, &flagsArg, &encodingArg, &doubleEncodeArg)
	if err != nil {
		return nil, err
	}

	if encodingArg.HasArg() {
		enc := string(encodingArg.Get())
		if !isSupportedHtmlCharset(enc) && enc != "" {
			ctx.Warn("Charset \"%s\" is not supported, assuming UTF-8", enc)
		}
	}

	flags := flagsArg.GetOrDefault(ENT_COMPAT | ENT_SUBSTITUTE | ENT_HTML401)
	doubleEncode := bool(doubleEncodeArg.GetOrDefault(true))

	escape := map[string]string{}
	for _, e := range getHtmlTranslationTable(HTML_SPECIALCHARS, flags) {
		escape[e.key] = e.value
	}

	var buf bytes.Buffer
	chars := []rune(str)
	for i := 0; i < len(chars); i++ {
		c := chars[i]

		if c == '&' && !doubleEncode {
			valid := false
			var j int
			var sub string
			if k := slices.Index(chars[i:], ';'); k >= 0 {
				j = min(i+k+1, len(chars))
				sub = string(chars[i:j])
				_, valid = entitySet[sub]
			}
			if valid {
				i = j - 1
				buf.WriteString(sub)
				continue
			}
		}

		if repl, ok := escape[string(c)]; ok {
			end := min(i+len(repl)+1, len(str))
			if !doubleEncode && (string(str[i:end]) == repl) {
				i += len(repl) - 1
			}
			buf.WriteString(repl)
		} else {

			buf.WriteRune(c)
		}

	}

	return phpv.ZStr(buf.String()), nil
}

// > func string htmlspecialchars_decode ( string $string [, int $flags = ENT_QUOTES | ENT_SUBSTITUTE ] )
func fncHtmlSpecialCharsDecode(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var str phpv.ZString
	var flagsArg core.Optional[phpv.ZInt]
	_, err := core.Expand(ctx, args, &str, &flagsArg)
	if err != nil {
		return nil, err
	}

	flags := flagsArg.GetOrDefault(ENT_QUOTES | ENT_HTML_SUBSTITUTE_ERRORS)

	unescape := map[string]string{}
	for _, e := range getHtmlTranslationTable(HTML_SPECIALCHARS, flags) {
		unescape[e.value] = e.key
	}

	var buf bytes.Buffer
	chars := []rune(str)
	for i := 0; i < len(chars); i++ {
		c := chars[i]

		if c != '&' {
			buf.WriteRune(c)
			continue
		}

		var j int
		if k := slices.Index(chars[i:], ';'); k <= 0 {
			buf.WriteRune(c)
			continue
		} else {
			j = min(i+k+1, len(chars))
		}
		sub := string(chars[i:j])

		var repl string
		if s, ok := unescape[sub]; !ok {
			buf.WriteRune(c)
			continue
		} else {
			repl = s
		}

		buf.WriteString(repl)
		i = j - 1
	}

	return phpv.ZStr(buf.String()), nil
}

// > func string htmlentities ( string $string [, int $flags = ENT_QUOTES | ENT_SUBSTITUTE | ENT_HTML401 [, string $encoding = ini_get("default_charset") [, bool $double_encode = TRUE ]]] )
func fncHtmlEntities(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var str phpv.ZString
	var flagsArg core.Optional[phpv.ZInt]
	var encodingArg core.Optional[phpv.ZString]
	var doubleEncodeArg core.Optional[phpv.ZBool]
	_, err := core.Expand(ctx, args, &str, &flagsArg, &encodingArg, &doubleEncodeArg)
	if err != nil {
		return nil, err
	}

	flags := flagsArg.GetOrDefault(ENT_QUOTES | ENT_SUBSTITUTE | ENT_HTML401)
	doubleEncode := bool(doubleEncodeArg.GetOrDefault(true))

	escape := map[string]string{}
	for _, e := range getHtmlTranslationTable(HTML_ENTITIES, flags) {
		escape[e.key] = e.value
	}

	var buf bytes.Buffer
	chars := []rune(str)
	for i := 0; i < len(chars); i++ {
		c := chars[i]

		if c == '&' && !doubleEncode {
			valid := false
			var j int
			var sub string
			if k := slices.Index(chars[i:], ';'); k >= 0 {
				j = min(i+k+1, len(chars))
				sub = string(chars[i:j])
				_, valid = entitySet[sub]
			}
			if valid {
				i = j - 1
				buf.WriteString(sub)
				continue
			}
		}

		if repl, ok := escape[string(c)]; ok {
			buf.WriteString(repl)
		} else {
			buf.WriteRune(c)
		}
	}

	return phpv.ZStr(buf.String()), nil
}

// > func string html_entity_decode ( string $string [, int $flags = ENT_QUOTES | ENT_SUBSTITUTE | ENT_HTML401 [, string $encoding = "UTF-8" ]] )
func fncHtmlEntityDecode(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var str phpv.ZString
	var flagsArg core.Optional[phpv.ZInt]
	var encodingArg core.Optional[phpv.ZString]
	_, err := core.Expand(ctx, args, &str, &flagsArg, &encodingArg)
	if err != nil {
		return nil, err
	}

	flags := flagsArg.GetOrDefault(ENT_QUOTES | ENT_SUBSTITUTE | ENT_HTML401)

	// Build reverse mapping from entity to character
	unescape := map[string]string{}
	for _, e := range getHtmlTranslationTable(HTML_ENTITIES, flags) {
		unescape[e.value] = e.key
	}

	// Also add special chars decode
	for _, e := range getHtmlTranslationTable(HTML_SPECIALCHARS, flags) {
		unescape[e.value] = e.key
	}

	docType := flags & ENT_HTML_DOC_TYPE_MASK

	var buf bytes.Buffer
	s := string(str)
	for i := 0; i < len(s); i++ {
		c := s[i]

		if c != '&' {
			buf.WriteByte(c)
			continue
		}

		// Find the ';'
		end := strings.IndexByte(s[i:], ';')
		if end < 0 {
			buf.WriteByte(c)
			continue
		}
		end += i

		entity := s[i : end+1]

		// Try named entity lookup
		if repl, ok := unescape[entity]; ok {
			buf.WriteString(repl)
			i = end
			continue
		}

		// Try numeric entity &#123; or &#x1A;
		if end > i+2 && s[i+1] == '#' {
			inner := s[i+2 : end]
			if len(inner) > 0 {
				var codepoint int64
				var parseErr error
				if inner[0] == 'x' || inner[0] == 'X' {
					if len(inner) > 1 {
						codepoint, parseErr = strconv.ParseInt(inner[1:], 16, 64)
					} else {
						parseErr = fmt.Errorf("empty hex")
					}
				} else {
					codepoint, parseErr = strconv.ParseInt(inner, 10, 64)
				}
				if parseErr == nil && codepoint >= 0 {
					if isAllowedCodepoint(codepoint, docType) {
						buf.WriteRune(rune(codepoint))
						i = end
						continue
					}
				}
			}
		}

		// Not decoded, output as-is
		buf.WriteByte(c)
	}

	return phpv.ZStr(buf.String()), nil
}

// isAllowedCodepoint checks if a numeric entity codepoint should be decoded
// based on the document type. Different doc types have different rules about
// which codepoints are allowed.
func isAllowedCodepoint(cp int64, docType phpv.ZInt) bool {
	// Null is never allowed
	if cp == 0 {
		return false
	}

	// C0 range (1-31): only certain chars allowed depending on doc type
	if cp >= 1 && cp <= 31 {
		if docType == ENT_HTML5 {
			// HTML5: allow 0x09, 0x0A, 0x0C but NOT 0x0D
			return cp == 0x09 || cp == 0x0A || cp == 0x0C
		}
		// HTML 4.01, XML 1.0, XHTML: allow 0x09, 0x0A, 0x0D
		return cp == 0x09 || cp == 0x0A || cp == 0x0D
	}

	// DEL (0x7F)
	if cp == 0x7F {
		if docType == ENT_HTML401 || docType == ENT_HTML5 {
			return false
		}
		return true // XML1, XHTML allow it
	}

	// C1 range (0x80-0x9F)
	if cp >= 0x80 && cp <= 0x9F {
		if docType == ENT_HTML401 || docType == ENT_HTML5 {
			return false
		}
		return true // XML1, XHTML allow it
	}

	// Surrogates (0xD800-0xDFFF) - never allowed
	if cp >= 0xD800 && cp <= 0xDFFF {
		return false
	}

	// Noncharacters
	if docType == ENT_HTML5 {
		// HTML5 forbids noncharacters
		if cp >= 0xFDD0 && cp <= 0xFDEF {
			return false
		}
		if cp&0xFFFF == 0xFFFE || cp&0xFFFF == 0xFFFF {
			return false
		}
	} else if docType == ENT_XHTML || docType == ENT_XML1 {
		// XHTML and XML 1.0 forbid 0xFFFE and 0xFFFF
		if cp == 0xFFFE || cp == 0xFFFF {
			return false
		}
	}

	return true
}

// isSupportedHtmlCharset checks if a charset name is supported by html functions
func isSupportedHtmlCharset(charset string) bool {
	upper := strings.ToUpper(strings.ReplaceAll(charset, "-", ""))
	supported := map[string]bool{
		"UTF8":        true,
		"ISO88591":    true,
		"ISO885915":   true,
		"ISO88595":    true,
		"CP1252":      true,
		"WINDOWS1252": true,
		"1252":        true,
		"CP1251":      true,
		"WINDOWS1251": true,
		"1251":        true,
		"CP866":       true,
		"866":         true,
		"IBM866":      true,
		"KOI8R":       true,
		"KOI8RU":      true,
		"MACROMAN":    true,
		"SJIS":        true,
		"SHIFTJIS":    true,
		"932":         true,
		"EUCJP":       true,
		"BIG5":        true,
		"950":         true,
		"GB2312":      true,
		"ASCII":       true,
	}
	return supported[upper]
}
