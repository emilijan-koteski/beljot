// Beljot · Rules content — Croatian (authored; ijekavian, Bela terminology).
import type { RulesLangData } from "./types";

export const hr: RulesLangData = {
  cardNames: {
    J: "Dečko",
    "9": "Devetka",
    A: "As",
    "10": "Desetka",
    K: "Kralj",
    Q: "Dama",
    "8": "Osmica",
    "7": "Sedmica",
  },
  trumpNotes: { J: "Najjača u adutu", "7": "Najslabija" },
  plainNotes: { A: "Najjača izvan aduta", "7": "Najslabija" },

  declarations: {
    belot: {
      name: "Bela-Rebela",
      summary: "Kralj i Dama u adutu, obje karte u istoj ruci.",
      detail:
        "Dama je Bela, Kralj je Rebela. Svaku zoveš kad je odigraš, a par nosi +20 bodova za tim.",
    },
    terca: {
      name: "Terca",
      summary: "Tri karte u nizu, sve iste boje.",
      detail:
        "Za zvanja redoslijed ide sedmica, osmica, devetka, desetka, Dečko, Dama, Kralj, As. Nema vraćanja od Asa natrag na sedmicu.",
    },
    kvarta: {
      name: "Kvarta",
      summary: "Četiri karte u nizu, sve iste boje.",
      detail: "Kvarta uvijek pobjeđuje bilo koju tercu koju drži drugi tim, bez obzira na boje.",
    },
    kvinta: {
      name: "Kvinta",
      summary: "Pet ili više karata u nizu, ista boja.",
      detail:
        "Kvinta uvijek pobjeđuje bilo koju kvartu koju drži drugi tim, bez obzira na boje. Bilo koji niz od pet ili više u jednoj boji vrijedi 100.",
    },
    carre: {
      name: "Četiri iste",
      summary: "Sve četiri iste vrijednosti, samo Desetke, Dame, Kraljevi ili Asovi.",
      detail:
        "Četiri iste od jedne od ovih vrijednosti. Četiri devetke i četiri dečka nose više i boduju se zasebno.",
    },
    carre9: {
      name: "Četiri devetke",
      summary: "Sve četiri devetke.",
      detail:
        "Devetka u adutu je druga najjača karta u špilu, pa četiri devetke vrijede jedan i pol put više od običnog zvanja četiri iste.",
    },
    carreJ: {
      name: "Četiri dečka",
      summary: "Sva četiri dečka.",
      detail:
        "Najveće pojedinačno zvanje u igri. Dobiti sva četiri dečka u svojih osam karata je rijetko. Većina igrača to vidi tek nekoliko puta u cijeloj sezoni.",
    },
    bela: {
      name: "Bela",
      summary: "Svih osam karata adutske boje, u jednoj ruci.",
      detail:
        "Najrjeđa ruka u igri. Svih osam karata aduta kod jednog igrača. Odmah nosi cijeli meč: čim je adut određen i ruke su pune, taj tim je proglašen pobjednikom, meč završava tu i ne igra se ni jedna karta.",
    },
  },

  sections: [
    {
      id: "goal",
      label: "Cilj",
      title: "Utrkuj se s timom do 1001",
      lede: "Ti i tvoj partner dijelite jedan rezultat. Prvi tim do 1001 osvaja meč.",
      blocks: [
        {
          kind: "p",
          text: "Sjediš nasuprot svom partneru, vas dvoje protiv para s obje strane. Dijelite jedan zajednički rezultat i ništa se ne resetira između odigranih ruku. Bodovi se samo gomilaju dok netko ne prijeđe 1001. Većina mečeva završi u 6 do 12 ruku.",
        },
        {
          kind: "p",
          text: "Postoje dva načina da osvojiš bodove. Osvoji štihove i skupljaš bodove ispisane na svakoj karti koju uzmeš. Drži prave karte i možeš zvati nizove od četiri u jednoj boji, ili Kralja i Damu u adutu zajedno i sl. Štihovi su tvoj stalni prihod, a zvanja su veliki preokreti koji znaju promijeniti tok cijelog meča.",
        },
      ],
    },
    {
      id: "basics",
      label: "Priprema",
      title: "Promiješaj, podijeli, zovi adut",
      lede: "Četiri igrača, 32 karte, osam u ruci i brzi krug da se odredi koja je boja adut.",
      blocks: [
        // Jedan blok, ne po jedan za svaku varijantu. Prva dva koraka identična
        // su u oba pravila i napisana su jednom; sljedeća četiri označena su po
        // varijanti, svaki sa svojom napomenom o svom paru.
        {
          kind: "steps",
          items: [
            {
              t: "Sjedni na svoje mjesto",
              d: "Sjediš točno nasuprot svom partneru; dvojica protivnika zauzimaju stolice s obje strane. Igra se kreće udesno oko stola.",
            },
            {
              t: "Sastavi špil",
              d: "Bela se igra s 32 karte. Uzmi obični špil i izbaci sve od 2 do 6. Ono što ostaje su sedmica, osmica, devetka, desetka, Dečko, Dama, Kralj i As u sve četiri boje. Time igraš.",
            },
            {
              variant: "bitola",
              otherVariantNote:
                "U hrvatskim pravilima svih osam karata dijeli se prije nego što se zove adut, po tri, tri, pa posljednje dvije licem prema dolje, koje ostaju skrivene dok se adut ne odredi, i ništa se ne okreće.",
              t: "Podijeli po pet, pa okreni jednu",
              d: "Djelitelj obilazi dvaput, po tri karte pa dvije, pa svatko počinje s pet u ruci. Sljedeća karta sa špila ide licem prema gore na stol kao adutska karta, a jedanaest iza nje ostaju licem prema dolje na sredini.",
            },
            {
              variant: "croatia",
              otherVariantNote:
                "U bitolskim pravilima dijeljenje staje na pet karata po igraču, a sljedeća karta se okreće kao adutska, dok ostalih jedanaest ostaje na sredini.",
              t: "Podijeli svih osam odmah",
              d: "Djelitelj obilazi triput: po tri karte, još tri, pa posljednje dvije licem prema dolje. Svaka je karta podijeljena prije nego što bilo tko zove adut. Šest možeš gledati, a dvije ne može nitko, ni ti sam.",
            },
            {
              variant: "bitola",
              otherVariantNote:
                "U hrvatskim pravilima nema karte za uzimanje i zove se samo jedan krug: svaki igrač naziva bilo koju od četiri boje, na temelju šest karata koje vidi, i ne dobiva dodatnu kartu.",
              t: "Prvi krug: uzmi tu kartu ili propusti",
              d: "Počevši zdesna od djelitelja, svaki igrač ili uzima okrenutu kartu, čime njezina boja postaje adut za tu ruku, ili propušta. Tko je uzme, zadržava je kao jednu od svojih osam, a djelitelj dijeli dok svaka ruka nije puna. Adut pobjeđuje sve iz druge tri boje, bez obzira na rang.",
            },
            {
              variant: "croatia",
              otherVariantNote:
                "U bitolskim pravilima prvi krug nudi tu jednu okrenutu kartu: tko je uzme, njezina boja postaje adut, a karta mu ostaje kao jedna od osam.",
              t: "Jedan krug: nazovi boju ili reci „dalje“",
              d: "Ništa se ne okreće i nema adutske karte za uzimanje. Počevši zdesna od djelitelja, svaki igrač redom ili naziva bilo koju od četiri boje kao adut, na temelju šest karata koje vidi, ili propušta s „dalje“. Tko nazove boju ne dobiva dodatnu kartu, ruka mu je već podijeljena. Adut pobjeđuje sve iz druge tri boje, bez obzira na rang.",
            },
            {
              variant: "bitola",
              otherVariantNote:
                "U hrvatskim pravilima drugog kruga nema: nakon tri propuštanja izbor pada na djelitelja, koji zove posljednji i mora nazvati boju — „pod mus“.",
              t: "Drugi krug: nazovi boju, ali ne tu",
              d: "Sva četvorica su propustila? Isti red ide ponovno, ali ovaj put boju nazivaš umjesto da uzimaš kartu. Boja okrenute karte je potrošena i zaključana, pa birate između druge tri. Onaj tko zove ipak uzima okrenutu kartu s ostatkom dijeljenja.",
            },
            {
              variant: "croatia",
              otherVariantNote:
                "U bitolskim pravilima djelitelj nikad nije prisiljen: ako sva četvorica propuste, otvara se drugi krug u kojem se naziva bilo koja boja osim one okrenute karte, a propadne li i on, dijeli se ispočetka.",
              t: "Djelitelj zove posljednji — „pod mus“",
              d: "Djelitelj zove četvrti i nema pravo propustiti: ako su druga trojica rekla „dalje“, djelitelj mora nazvati boju, i dalje gledajući samo svojih šest karata. Taj prisilni izbor aduta poznat je kao igranje „pod mus“. U ovim pravilima nema drugog kruga ni ponovnog miješanja, svako dijeljenje se igra.",
            },
            {
              variant: "bitola",
              otherVariantNote:
                "U hrvatskim pravilima nijedno dijeljenje ne propada: djelitelj u jedinom krugu zove posljednji i mora nazvati boju, pa se svako dijeljenje igra.",
              t: "Dvaput propušteno? Novo dijeljenje",
              d: "Ako i drugi krug prođe bez ijednog zvanja aduta, ruka se ne igra: sve 32 karte vraćaju se zajedno i miješaju, dijeljenje prelazi na sljedećeg igrača zdesna, i sve počinje ispočetka.",
            },
            {
              variant: "croatia",
              otherVariantNote:
                "U bitolskim pravilima ništa se ne dijeli licem prema dolje, pa nema što otkrivati: ruka onoga tko uzme adut popunjava se s talona, a dvaput propušteno dijeljenje miješa se ispočetka.",
              t: "Posljednje dvije se otkrivaju",
              d: "Tek kad je adut nazvan, dvije karte licem prema dolje okreću se, svakom igraču samo njegove. Nitko ne vidi tuđe. Od tog trenutka svi drže svih osam karata i ruka se igra do kraja.",
            },
          ],
        },
      ],
    },
    {
      id: "cards",
      label: "Vrijednost karata",
      title: "Adut igra po vlastitim pravilima",
      lede: "U adutu, Dečko i devetka postaju najjači. Za sve druge boje vrijedi redoslijed izvan aduta.",
      blocks: [
        {
          kind: "p",
          text: "Svaka karta radi dvije stvari. Njezina snaga određuje tko nosi štih; njezina vrijednost u bodovima dodaje se tvom rezultatu ako je osvojiš. To dvoje nije uvijek isto. Karta može biti jaka a ništa ne vrijediti, ili slaba a nositi puno bodova.",
        },
        {
          kind: "p",
          text: "U tri obične boje, redoslijed je poznat: As na vrhu, pa desetka, Kralj, Dama, Dečko i naniže. No čim jedna boja postane adut, dvije karte skaču gore. Dečko u adutu postaje najjača karta u cijelom špilu, a devetka u adutu odmah iza njega. As i desetka u adutu padaju na treće i četvrto mjesto. Brzo prebacivanje između ova dva redoslijeda najveći je dio igre.",
        },
        { kind: "cards" },
        {
          kind: "note",
          text: "Zbroji sve karte u špilu i dobiješ 152 boda. Osvoji posljednji štih u ruci i uzimaš još 10 (bonus za „posljednji štih“), pa je na stolu 162 boda u svakoj ruci prije nego što se dodaju zvanja.",
        },
      ],
    },
    {
      id: "play",
      label: "Igranje štiha",
      title: "Kada što smiješ baciti",
      lede: "Rijetko si slobodan baciti što želiš. Tri kratka pravila pokrivaju gotovo svaki potez.",
      blocks: [
        {
          kind: "p",
          text: "Štih je po jedna karta od svakog od četvorice igrača, redom. Tko nosi štih skuplja sve četiri karte u hrpu svog tima i vodi sljedeći. Osam štihova i ruka je gotova.",
        },
        {
          kind: "rule",
          title: "Prati boju koja je izašla i nadmaši je ako možeš",
          text: "Ako je izašao Herc, moraš baciti Herc kad god ga imaš. I ne smiješ se izvući: ako držiš Herc veći od najvećeg koji je već na stolu, dužan si ga baciti. Tek kad su svi tvoji manji smiješ pustiti manjeg.",
        },
        {
          kind: "rule",
          title: "Nemaš u boji? Moraš rezati i nadmašiti ako možeš",
          text: "Ne možeš pratiti boju ali još držiš adut? Dužan si rezati. I ako je adut već bačen, moraš ga nadmašiti većim kada možeš; samo ako su svi tvoji aduti manji smiješ baciti mali adut. Najveći adut na stolu nosi štih.",
        },
        {
          kind: "rule",
          title: "Rezano adutom? Praćenje boje ipak je prvo",
          text: "Kad je štih već rezan adutom, i dalje moraš pratiti izašlu boju ako je imaš, ali bilo koja karta te boje je dovoljna, jer adut već nosi štih i tvoja boja ga više ne može osvojiti. Za adut posežeš samo kad uopće nemaš izašlu boju; a ako je netko prije tebe već rezao, moraš nadmašiti njegov adut višim ako možeš, ili baciti bilo koji adut ako ne možeš.",
        },
        {
          kind: "p",
          text: "Nemaš kartu izašle boje ni adut? Baci što želiš. Ta karta ne može osvojiti štih, samo je pokupi onaj tko ga nosi.",
        },
      ],
    },
    {
      id: "melds",
      label: "Zvanja",
      title: "Neke ruke nose bodove same po sebi",
      lede: "Imaj pravu kombinaciju u podijeljenoj ruci i ona nosi bodove sama po sebi, uz ono što vrijede tvoji štihovi.",
      blocks: [
        {
          kind: "p",
          text: "Čim su karte podijeljene i adut određen, provjeri ruku za zvanja: nizove karata u nizu u jednoj boji, četiri iste, i par Kralj-i-Dama u adutu. Dama je Bela, Kralj je Rebela. Bela i Rebela su iznimka. Svaku zoveš kad igraš tu kartu, u kojem se god dijelu ruke to dogodi.",
        },
        {
          kind: "p",
          variant: "bitola",
          otherVariantNote:
            "U hrvatskim pravilima zvanja imaju vlastitu fazu između zvanja aduta i prvog štiha: svako mjesto zove ili preskače, cijeli se stol otkriva odjednom i tek onda se igra karta.",
          text: "Nema odvojenog kruga za zvanja. Zoveš na svom redu u prvom štihu, dok igraš kartu, a zatim slažeš karte licem prema gore za sve na početku drugog štiha.",
        },
        {
          kind: "p",
          variant: "croatia",
          otherVariantNote:
            "U bitolskim pravilima nema odvojene faze: zoveš na svom redu u prvom štihu, dok igraš kartu, a karte idu licem prema gore na početku drugog.",
          text: "Zvanja imaju vlastitu fazu, između zvanja aduta i prvog štiha. Svako mjesto redom zove ili preskače, zvanja cijelog stola se zatim otkrivaju zajedno, i tek nakon toga počinje prvi štih.",
        },
        { kind: "melds" },
        {
          kind: "rule",
          title: "Samo jedan tim je plaćen za zvanja",
          text: "Svaka strana ističe svoje jedino najbolje zvanje. Čije je jače, skuplja sva zvanja iz obje ruke tima, a drugi tim ne dobiva ništa za svoja. Jednako vrijede? Četiri iste pobjeđuju niz, pa četiri asa (100) pobjeđuju kvintu (100). Dulji niz pobjeđuje kraći, ali samo do kvinte. Kad obje strane imaju kvintu (pet ili više karata), duljina više ne znači ništa, nego pobjeđuje niz s višom gornjom kartom, jednako kao i kod dva niza iste duljine. Još uvijek izjednačeno? Niz u adutu nosi. A ako nijedan niz nije adut, prednost ima onaj od dvojice igrača koji je prije na redu, počevši zdesna od djelitelja. Bela i Rebela stoje izvan ovog natjecanja, tko ih zove, uvijek ih boduje.",
        },
        {
          kind: "rule",
          variant: "bitola",
          otherVariantNote:
            "U hrvatskim pravilima karta nema takvo ograničenje: računa se u svako zvanje kojem pripada, pa se dvije kombinacije koje dijele kartu boduju u cijelosti.",
          title: "Jedna karta, jedno zvanje",
          text: "Jedna karta ne može se računati dvaput. Ako dvije tvoje kombinacije traže istu kartu, kao terca u Hercu i četiri kralja koje oboje žele tvog Kralja Herca, računa se samo vrjednija, a druga pada. Jednako vrijede? Ostaju četiri iste.",
        },
        {
          kind: "rule",
          variant: "croatia",
          otherVariantNote:
            "U bitolskim pravilima jedna karta računa se samo u jedno zvanje: od dvije kombinacije koje dijele kartu pada slabija, a pri jednakoj vrijednosti ostaju četiri iste.",
          title: "Jedna karta može se računati više puta",
          text: "Ista karta može pripadati nekoliko zvanja istodobno i svako od njih boduje se u cijelosti. Terca u Hercu i četiri kralja koje oboje računaju tvog Kralja Herca je u redu. Zoveš oboje i ništa se ne umanjuje.",
        },
      ],
    },
    {
      id: "scoring",
      label: "Bodovanje",
      title: "Brojanje i zamka",
      lede: "Onaj tko je zvao adut daje obećanje: prođi, ili predaj protivnicima sve što si osvojio te ruke.",
      blocks: [
        {
          kind: "steps",
          items: [
            {
              t: "Prebroji karte koje si uzeo",
              d: "Svaki tim okreće osvojene štihove i zbraja bodove na kartama unutra. Zbirno za oba tima uvijek izlazi točno 152.",
            },
            {
              t: "Dodaj bonus za posljednji štih",
              d: "Osvojio osmi i posljednji štih? To je još 10 bodova, za stolom ga zovu „di de der“. Sada si na 162 samo od karata.",
            },
            {
              t: "Dodaj zvanja",
              d: "Strana koja je dobila natjecanje zvanja zbraja sve kombinacije iz ruku oba partnera. Bilo koja Bela ili Rebela zvana tijekom igre dolazi povrh toga, za onoga tko ju je zvao.",
            },
          ],
        },
        {
          kind: "rule",
          title: "Onaj tko je zvao adut mora proći",
          text: "Tim koji je zvao adut mora završiti sa strogo više bodova od druge strane, uključujući zvanja s obje strane. Ako zaostane ili se čak izjednači, ruka je izgubljena: sve što je osvojio te ruke, i karte i zvanja, ide protivnicima umjesto toga. Igrači to zovu „pad“, i jedna loša ruka može izbrisati udobnu prednost.",
        },
        {
          kind: "rule",
          title: "Svih osam štihova je štiglja",
          text: "Osvoji sve štihove u ruci i bonus od 10 bodova za posljednji štih zamjenjuje se sa 100, jer je posljednji štih ionako tvoj. Tvoj tim tada uzima sve bodove na stolu: bodove s karata, taj bonus i zvanja obaju timova, uključujući i protivnička. Tim koji nije osvojio ni jedan štih ne boduje ništa, ni vlastita zvanja.",
        },
        {
          kind: "note",
          text: "Ruke se igraju dok barem jedan tim ne sjedne na 1001 ili više na kraju ruke. Ako oba tima prijeđu granicu u istoj ruci, strana s više ukupnih bodova osvaja meč. A ako su oba zbroja točno jednaka, meč osvaja tim koji je zvao adut. Za kraći meč, soba se može postaviti i na utrku do 501 boda, uz potpuno ista pravila.",
        },
      ],
    },
    {
      id: "honour",
      label: "Čast",
      title: "Završi ono što započneš",
      lede: "Bela je igra u parovima. Igrač koji napusti meč u tijeku pokvari ga za još tri igrača. Čast pokazuje koliko si pouzdan suigrač.",
      blocks: [
        {
          kind: "p",
          text: "Ocjena časti je udio mečeva koje si završio, pri čemu noviji mečevi vrijede mnogo više od stare povijesti. Novi igrač počinje na 80, a ocjena se ne prikazuje dok ne odigra 5 mečeva, jer prije toga nema što mjeriti.",
        },
        {
          kind: "steps",
          items: [
            {
              t: "Završen meč podiže ocjenu",
              d: "Bez obzira na pobjedu ili poraz. Čast ne mjeri vještinu, samo jesi li bio za stolom do kraja.",
            },
            {
              t: "Predaja se i dalje računa kao završeno",
              d: "Predan meč je završen meč. Dogovor s partnerom da se meč završi suprotnost je napuštanju i podiže ocjenu kao i svaki drugi završetak.",
            },
            {
              t: "Napuštanje bez vraćanja je snižava",
              d: "To je jedino što je snižava. Ako izgubiš vezu, imaš puni prozor za ponovno povezivanje, a vraćanje te ne košta ništa.",
            },
            {
              t: "Vrijeme je popravlja",
              d: "Stara napuštanja blijede. Loš niz od prije nekoliko mjeseci vrijedi vrlo malo prema mečevima koje si nedavno završio.",
            },
          ],
        },
        {
          kind: "tiers",
          title: "Pet razina",
          items: [
            { tier: "exemplary", d: "gotovo nikad ne odustaje." },
            { tier: "trusted", d: "je pouzdan partner." },
            { tier: "fair", d: "je uobičajena razina." },
            { tier: "unreliable", d: "zaključan je za neke stolove." },
            { tier: "problematic", d: "zaključan je za većinu stolova." },
          ],
          text: "Domaćin sobe može postaviti minimum i posebno odlučiti jesu li dobrodošli igrači koji još nemaju ocjenu.",
        },
        {
          kind: "note",
          text: "Dvije stvari koje igrači najčešće pogrešno razumiju: predaja ne šteti tvojoj časti, i jedna loša večer ne prati te zauvijek. Ponovno povezivanje nakon prekida veze uvijek je besplatno.",
        },
      ],
    },
  ],

  ui: {
    heroEyebrow: "Pravila · čitanje od 6 minuta",
    heroTitle: "Nauči Belu u jednom sjedenju",
    heroIntro:
      "Bela je timska igra s kartama za četiri igrača sa špilom od 32 karte. Šest kratkih poglavlja u nastavku vode te od prve ruke sve do pobjedničkog rezultata, sve što ti treba da se snađeš za stolom. Čitaj redom, ili skoči na ono što ti treba preko sadržaja lijevo.",
    facts: [
      { label: "Igrači", value: "4", caption: "dva tima po dvoje" },
      { label: "Špil", value: "32", caption: "od sedmice do Asa, četiri boje" },
      { label: "Karte po ruci", value: "8", caption: "osam u ruci prije prvog štiha" },
      { label: "Utrka do", value: "1001", caption: "bodova za pobjedu" },
    ],
    tocTitle: "Sadržaj",
    footerTitle: "Spreman za prvu ruku?",
    footerBody:
      "Ovaj vodič prati te i u igru. Tijekom ruke, pritisni gumb s upitnikom u donjem desnom kutu i istih šest poglavlja se otvara, bez pauziranja igre.",
    footerCta: "Igraj",
    noteLabel: "Napomena",
    variantLabel: "Varijanta",
    diffLabel: "Razlikuje se u drugoj varijanti",
    pts: "bodova",
    ladderTrumpTitle: "U adutskoj boji",
    ladderTrumpEyebrow: "Adut",
    ladderPlainTitle: "U svakoj drugoj boji",
    ladderPlainEyebrow: "Izvan aduta",
    colCard: "Karta",
    colPoints: "Bodovi",
    colPower: "Snaga",
    meldKinds: { belot: "Par u adutu", set: "Četiri iste", run: "Niz" },
    ovReference: "Uputa",
    ovTitle: "Pravila Bele",
    ovChapters: "Poglavlja",
    ovFullRef: "Potpuna uputa:",
    ovClose: "Zatvori",
  },
};
