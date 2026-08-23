// Beljot · Rules content — Serbian (authored; ekavian, Latin, Bela terminology).
import type { RulesLangData } from "./types";

export const sr: RulesLangData = {
  cardNames: {
    J: "Žandar",
    "9": "Devetka",
    A: "As",
    "10": "Desetka",
    K: "Kralj",
    Q: "Dama",
    "8": "Osmica",
    "7": "Sedmica",
  },
  trumpNotes: { J: "Najjača u adutu", "7": "Najslabija" },
  plainNotes: { A: "Najjača van aduta", "7": "Najslabija" },

  declarations: {
    belot: {
      name: "Bela-Rebela",
      summary: "Kralj i Dama u adutu, obe karte u istoj ruci.",
      detail:
        "Dama je Bela, Kralj je Rebela. Svaku zoveš kad je odigraš, a par nosi +20 poena za tim.",
    },
    terca: {
      name: "Terca",
      summary: "Tri karte u nizu, sve iste boje.",
      detail:
        "Za zvanja redosled ide sedmica, osmica, devetka, desetka, Žandar, Dama, Kralj, As. Nema vraćanja od Asa nazad na sedmicu.",
    },
    kvarta: {
      name: "Kvarta",
      summary: "Četiri karte u nizu, sve iste boje.",
      detail: "Kvarta uvek pobeđuje bilo koju tercu koju drži drugi tim, bez obzira na boje.",
    },
    kvinta: {
      name: "Kvinta",
      summary: "Pet ili više karata u nizu, ista boja.",
      detail:
        "Kvinta uvek pobeđuje bilo koju kvartu koju drži drugi tim, bez obzira na boje. Bilo koji niz od pet ili više u jednoj boji vredi 100.",
    },
    carre: {
      name: "Kare",
      summary: "Sve četiri iste vrednosti, samo Desetke, Dame, Kraljevi ili Asovi.",
      detail:
        "Četiri iste od jedne od ovih vrednosti. Karei od devetki i žandara nose više i boduju se posebno.",
    },
    carre9: {
      name: "Kare devetki",
      summary: "Sve četiri devetke.",
      detail:
        "Devetka u adutu je druga najjača karta u špilu, pa pun kare devetki plaća se jedan i po put više od običnog karea.",
    },
    carreJ: {
      name: "Kare žandara",
      summary: "Sva četiri žandara.",
      detail:
        "Najveće pojedinačno zvanje u igri. Dobiti sva četiri žandara u svojih osam karata je retko. Većina igrača to vidi tek nekoliko puta u celoj sezoni.",
    },
    bela: {
      name: "Bela",
      summary: "Svih osam karata adutske boje, u jednoj ruci.",
      detail:
        "Najređa ruka u igri. Svih osam karata aduta kod jednog igrača. Odmah nosi ceo meč: čim je adut određen i ruke su pune, taj tim je proglašen pobednikom, meč završava tu i ne igra se ni jedna karta.",
    },
  },

  sections: [
    {
      id: "goal",
      label: "Cilj",
      title: "Trkaj se s timom do 1001",
      lede: "Ti i tvoj partner delite jedan rezultat. Prvi tim do 1001 osvaja meč.",
      blocks: [
        {
          kind: "p",
          text: "Sediš naspram svog partnera, vas dvoje protiv para s obe strane. Delite jedan zajednički rezultat i ništa se ne resetuje između odigranih ruku. Poeni se samo gomilaju dok neko ne pređe 1001. Većina mečeva završi za 6 do 12 ruku.",
        },
        {
          kind: "p",
          text: "Postoje dva načina da osvojiš poene. Osvoji štihove i skupljaš poene ispisane na svakoj karti koju uzmeš. Drži prave karte i možeš da zoveš nizove od četiri u jednoj boji, ili Kralja i Damu u adutu zajedno i sl. Štihovi su tvoj stalni prihod, a zvanja su veliki preokreti koji znaju da promene tok celog meča.",
        },
      ],
    },
    {
      id: "basics",
      label: "Priprema",
      title: "Promešaj, podeli, zovi adut",
      lede: "Četiri igrača, 32 karte, osam u ruci i brz krug da se odredi koja je boja adut.",
      blocks: [
        // Jedan blok, ne po jedan za svaku varijantu. Prva dva koraka identična
        // su u oba pravila i napisana su jednom; sledeća četiri označena su po
        // varijanti, svaki sa svojom napomenom o svom paru.
        {
          kind: "steps",
          items: [
            {
              t: "Sedni na svoje mesto",
              d: "Sediš tačno naspram svog partnera; dvojica protivnika zauzimaju stolice s obe strane. Igra se kreće udesno oko stola.",
            },
            {
              t: "Sastavi špil",
              d: "Bela se igra sa 32 karte. Uzmi običan špil i izbaci sve od 2 do 6. Ono što ostaje su sedmica, osmica, devetka, desetka, Žandar, Dama, Kralj i As u sve četiri boje. Time igraš.",
            },
            {
              variant: "bitola",
              otherVariantNote:
                "U hrvatskim pravilima svih osam karata deli se pre nego što se zove adut, po tri, tri, pa poslednje dve licem nadole, koje ostaju skrivene dok se adut ne odredi, i ništa se ne okreće.",
              t: "Podeli po pet, pa okreni jednu",
              d: "Delilac obilazi dvaput, po tri karte pa dve, pa svako počinje sa pet u ruci. Sledeća karta sa špila ide licem nagore na sto kao adutska karta, a jedanaest iza nje ostaju licem nadole na sredini.",
            },
            {
              variant: "croatia",
              otherVariantNote:
                "U bitolskim pravilima deljenje staje na pet karata po igraču, a sledeća karta se okreće kao adutska, dok ostalih jedanaest ostaje na sredini.",
              t: "Podeli svih osam odmah",
              d: "Delilac obilazi triput: po tri karte, još tri, pa poslednje dve licem nadole. Svaka je karta podeljena pre nego što bilo ko zove adut. Šest možeš da gledaš, a dve ne može niko, ni ti sam.",
            },
            {
              variant: "bitola",
              otherVariantNote:
                "U hrvatskim pravilima nema karte za uzimanje i zove se samo jedan krug: svaki igrač naziva bilo koju od četiri boje, na osnovu šest karata koje vidi, i ne dobija dodatnu kartu.",
              t: "Prvi krug: uzmi tu kartu ili preskoči",
              d: "Počev zdesna od delioca, svaki igrač ili uzima okrenutu kartu, čime njena boja postaje adut za tu ruku, ili preskače. Ko je uzme, zadržava je kao jednu od svojih osam, a delilac deli dok svaka ruka nije puna. Adut pobeđuje sve iz druge tri boje, bez obzira na rang.",
            },
            {
              variant: "croatia",
              otherVariantNote:
                "U bitolskim pravilima prvi krug nudi tu jednu okrenutu kartu: ko je uzme, njena boja postaje adut, a karta mu ostaje kao jedna od osam.",
              t: "Jedan krug: nazovi boju ili reci „dalje“",
              d: "Ništa se ne okreće i nema adutske karte za uzimanje. Počev zdesna od delioca, svaki igrač redom ili naziva bilo koju od četiri boje kao adut, na osnovu šest karata koje vidi, ili preskače sa „dalje“. Ko nazove boju ne dobija dodatnu kartu, ruka mu je već podeljena. Adut pobeđuje sve iz druge tri boje, bez obzira na rang.",
            },
            {
              variant: "bitola",
              otherVariantNote:
                "U hrvatskim pravilima drugog kruga nema: posle tri preskakanja izbor pada na delioca, koji zove poslednji i mora da nazove boju — „pod mus“.",
              t: "Drugi krug: nazovi boju, ali ne tu",
              d: "Sva četvorica su preskočila? Isti red ide ponovo, ali ovaj put boju nazivaš umesto da uzimaš kartu. Boja okrenute karte je potrošena i zaključana, pa birate između druge tri. Onaj ko zove ipak uzima okrenutu kartu sa ostatkom deljenja.",
            },
            {
              variant: "croatia",
              otherVariantNote:
                "U bitolskim pravilima delilac nikad nije prisiljen: ako sva četvorica preskoče, otvara se drugi krug u kome se naziva bilo koja boja osim one okrenute karte, a ako propadne i on, deli se ispočetka.",
              t: "Delilac zove poslednji — „pod mus“",
              d: "Delilac zove četvrti i nema pravo da preskoči: ako su druga trojica rekla „dalje“, delilac mora da nazove boju, i dalje gledajući samo svojih šest karata. Taj prisilni izbor aduta poznat je kao igranje „pod mus“. U ovim pravilima nema drugog kruga ni ponovnog mešanja, svako deljenje se igra.",
            },
            {
              variant: "bitola",
              otherVariantNote:
                "U hrvatskim pravilima nijedno deljenje ne propada: delilac u jedinom krugu zove poslednji i mora da nazove boju, pa se svako deljenje igra.",
              t: "Dvaput preskočeno? Novo deljenje",
              d: "Ako i drugi krug prođe bez ijednog zvanja aduta, ruka se ne igra: sve 32 karte vraćaju se zajedno i mešaju, deljenje prelazi na sledećeg igrača zdesna, i sve počinje ispočetka.",
            },
            {
              variant: "croatia",
              otherVariantNote:
                "U bitolskim pravilima ništa se ne deli licem nadole, pa nema šta da se otkriva: ruka onoga ko uzme adut popunjava se sa talona, a dvaput preskočeno deljenje meša se ispočetka.",
              t: "Poslednje dve se otkrivaju",
              d: "Tek kad je adut nazvan, dve karte licem nadole se okreću, svakom igraču samo njegove. Niko ne vidi tuđe. Od tog trenutka svi drže svih osam karata i ruka se igra do kraja.",
            },
          ],
        },
      ],
    },
    {
      id: "cards",
      label: "Vrednost karata",
      title: "Adut igra po sopstvenim pravilima",
      lede: "U adutu, Žandar i devetka postaju najjači. Za sve druge boje važi redosled van aduta.",
      blocks: [
        {
          kind: "p",
          text: "Svaka karta radi dve stvari. Njena snaga određuje ko nosi štih; njena vrednost u poenima dodaje se tvom rezultatu ako je osvojiš. To dvoje nije uvek isto. Karta može biti jaka a da ništa ne vredi, ili slaba a da nosi mnogo poena.",
        },
        {
          kind: "p",
          text: "U tri obične boje, redosled je poznati: As na vrhu, pa desetka, Kralj, Dama, Žandar i naniže. Ali čim jedna boja postane adut, dve karte skaču nagore. Žandar u adutu postaje najjača karta u celom špilu, a devetka u adutu odmah iza njega. As i desetka u adutu padaju na treće i četvrto mesto. Brzo prebacivanje između ova dva redosleda najveći je deo igre.",
        },
        { kind: "cards" },
        {
          kind: "note",
          text: "Saberi sve karte u špilu i dobiješ 152 poena. Osvoji poslednji štih u ruci i uzimaš još 10 (bonus za „poslednji štih“), pa je na stolu 162 poena u svakoj ruci pre nego što se dodaju zvanja.",
        },
      ],
    },
    {
      id: "play",
      label: "Igranje štiha",
      title: "Kada šta smeš da baciš",
      lede: "Retko si slobodan da baciš šta želiš. Tri kratka pravila pokrivaju gotovo svaki potez.",
      blocks: [
        {
          kind: "p",
          text: "Štih je po jedna karta od svakog od četvorice igrača, redom. Ko nosi štih skuplja sve četiri karte u gomilu svog tima i vodi sledeći. Osam štihova i ruka je gotova.",
        },
        {
          kind: "rule",
          title: "Prati boju koja je izašla i nadbij je ako možeš",
          text: "Ako je izašao Herc, moraš da baciš Herc kad god ga imaš. I ne smeš da se izvučeš: ako držiš Herc veći od najvećeg koji je već na stolu, dužan si da ga baciš. Tek kad su svi tvoji manji smeš da pustiš manjeg.",
        },
        {
          kind: "rule",
          title: "Nemaš u boji? Moraš da sečeš i nadseci ako možeš",
          text: "Ne možeš da pratiš boju ali još držiš adut? Dužan si da sečeš. I ako je adut već bačen, moraš da ga nadbiješ većim kada možeš; samo ako su svi tvoji aduti manji smeš da baciš mali adut. Najveći adut na stolu nosi štih.",
        },
        {
          kind: "rule",
          title: "Sečeno adutom? Praćenje boje ipak je prvo",
          text: "Kad je štih već sečen adutom, i dalje moraš da pratiš izašlu boju ako je imaš, ali bilo koja karta te boje je dovoljna, jer adut već nosi štih i tvoja boja više ne može da ga osvoji. Za adut posežeš samo kad uopšte nemaš izašlu boju; a ako je neko pre tebe već sekao, moraš da nadsečeš njegov adut višim ako možeš, ili da baciš bilo koji adut ako ne možeš.",
        },
        {
          kind: "p",
          text: "Nemaš kartu izašle boje ni adut? Baci šta želiš. Ta karta ne može da osvoji štih, samo je pokupi onaj ko ga nosi.",
        },
      ],
    },
    {
      id: "melds",
      label: "Zvanja",
      title: "Neke ruke nose poene same po sebi",
      lede: "Imaj pravu kombinaciju u podeljenoj ruci i ona nosi poene sama po sebi, uz ono što vrede tvoji štihovi.",
      blocks: [
        {
          kind: "p",
          text: "Čim su karte podeljene i adut određen, proveri ruku za zvanja: nizove karata u nizu u jednoj boji, četiri iste, i par Kralj-i-Dama u adutu. Dama je Bela, Kralj je Rebela. Bela i Rebela su izuzetak. Svaku zoveš kad igraš tu kartu, u kom se god delu ruke to dogodi.",
        },
        {
          kind: "p",
          variant: "bitola",
          otherVariantNote:
            "U hrvatskim pravilima zvanja imaju sopstvenu fazu između zvanja aduta i prvog štiha: svako mesto zove ili preskače, ceo sto se otkriva odjednom i tek onda se igra karta.",
          text: "Nema odvojenog kruga za zvanja. Zoveš na svom redu u prvom štihu, dok igraš kartu, a zatim slažeš karte licem nagore za sve na početku drugog štiha.",
        },
        {
          kind: "p",
          variant: "croatia",
          otherVariantNote:
            "U bitolskim pravilima nema odvojene faze: zoveš na svom redu u prvom štihu, dok igraš kartu, a karte idu licem nagore na početku drugog.",
          text: "Zvanja imaju sopstvenu fazu, između zvanja aduta i prvog štiha. Sva četiri mesta biraju istovremeno i imaju osam sekundi da zovu ili preskoče, pa se ne vidi ko šta drži; zvanja celog stola se zatim otkrivaju zajedno, i tek nakon toga počinje prvi štih.",
        },
        { kind: "melds" },
        {
          kind: "rule",
          title: "Samo jedan tim je plaćen za zvanja",
          text: "Svaka strana ističe svoje jedino najbolje zvanje. Čije je jače, skuplja sva zvanja iz obe ruke tima, a drugi tim ne dobija ništa za svoja. Jednako vrede? Kare pobeđuje niz, pa kare asova (100) pobeđuje kvintu (100). Duži niz pobeđuje kraći, ali samo do kvinte. Kad obe strane imaju kvintu (pet ili više karata), dužina više ne znači ništa, nego pobeđuje niz sa višom gornjom kartom, isto kao i kod dva niza iste dužine. Još uvek izjednačeno? Niz u adutu nosi. A ako nijedan niz nije adut, prednost ima onaj od dvojice igrača koji je pre na redu, počev zdesna od delioca. Bela i Rebela stoje van ovog takmičenja, ko ih zove, uvek ih boduje.",
        },
        {
          kind: "rule",
          variant: "bitola",
          otherVariantNote:
            "U hrvatskim pravilima karta nema takvo ograničenje: računa se u svako zvanje kojem pripada, pa se dve kombinacije koje dele kartu boduju u celosti.",
          title: "Jedna karta, jedno zvanje",
          text: "Jedna karta ne može da se računa dvaput. Ako dve tvoje kombinacije traže istu kartu, kao terca u Hercu i kare kraljeva koji oboje žele tvog Kralja Herca, računa se samo vrednija, a druga pada. Jednako vrede? Ostaje kare.",
        },
        {
          kind: "rule",
          variant: "croatia",
          otherVariantNote:
            "U bitolskim pravilima jedna karta računa se samo u jedno zvanje: od dve kombinacije koje dele kartu pada slabija, a pri jednakoj vrednosti ostaje kare.",
          title: "Jedna karta može da se računa više puta",
          text: "Ista karta može da pripada nekolikim zvanjima istovremeno i svako od njih boduje se u celosti. Terca u Hercu i kare kraljeva koji oboje računaju tvog Kralja Herca je u redu. Zoveš oboje i ništa se ne umanjuje.",
        },
      ],
    },
    {
      id: "scoring",
      label: "Bodovanje",
      title: "Brojanje i zamka",
      lede: "Onaj ko je zvao adut daje obećanje: prođi, ili predaj protivnicima sve što si osvojio te ruke.",
      blocks: [
        {
          kind: "steps",
          items: [
            {
              t: "Prebroj karte koje si uzeo",
              d: "Svaki tim okreće osvojene štihove i sabira poene na kartama unutra. Zbirno za oba tima uvek izlazi tačno 152.",
            },
            {
              t: "Dodaj bonus za poslednji štih",
              d: "Osvojio osmi i poslednji štih? To je još 10 poena, za stolom ga zovu „di de der“. Sada si na 162 samo od karata.",
            },
            {
              t: "Dodaj zvanja",
              d: "Strana koja je dobila takmičenje zvanja sabira sve kombinacije iz ruku oba partnera. Bilo koja Bela ili Rebela zvana tokom igre dolazi povrh toga, za onoga ko ju je zvao.",
            },
          ],
        },
        {
          kind: "rule",
          title: "Onaj ko je zvao adut mora da prođe",
          text: "Tim koji je zvao adut mora da završi sa strogo više poena od druge strane, uključujući zvanja s obe strane. Ako zaostane ili se čak izjednači, ruka je izgubljena: sve što je osvojio te ruke, i karte i zvanja, ide protivnicima umesto toga. Igrači to zovu „pad“, i jedna loša ruka može da izbriše udobnu prednost.",
        },
        {
          kind: "rule",
          title: "Svih osam štihova je kapot",
          text: "Osvoji sve štihove u ruci i bonus od 10 poena za poslednji štih zamenjuje se sa 100, jer je poslednji štih ionako tvoj. Tvoj tim tada uzima sve poene na stolu: poene sa karata, taj bonus i zvanja oba tima, uključujući i protivnička. Tim koji nije osvojio ni jedan štih ne boduje ništa, ni sopstvena zvanja.",
        },
        {
          kind: "note",
          text: "Ruke se igraju dok bar jedan tim ne sedne na 1001 ili više na kraju ruke. Ako oba tima pređu granicu u istoj ruci, strana sa više ukupnih poena osvaja meč. A ako su oba zbira tačno jednaka, meč osvaja tim koji je zvao adut. Za kraći meč, soba se može podesiti i na trku do 501 poena, uz potpuno ista pravila.",
        },
      ],
    },
    {
      id: "honour",
      label: "Čast",
      title: "Završi ono što započneš",
      lede: "Bela je igra u parovima. Igrač koji napusti meč u toku kvari ga za još tri igrača. Čast pokazuje koliko si pouzdan saigrač.",
      blocks: [
        {
          kind: "p",
          text: "Skor časti je udeo mečeva koje si završio, s tim da skoriji mečevi vrede mnogo više od stare istorije. Nov igrač počinje na 80, a skor se ne prikazuje dok ne odigra 5 mečeva, jer pre toga nema šta da se meri.",
        },
        {
          kind: "steps",
          items: [
            {
              t: "Završen meč podiže skor",
              d: "Bez obzira na pobedu ili poraz. Čast ne meri veštinu, samo da li si bio za stolom do kraja.",
            },
            {
              t: "Predaja se i dalje računa kao završeno",
              d: "Predat meč je završen meč. Dogovor s partnerom da se meč završi je suprotnost napuštanju i podiže skor kao i svaki drugi završetak.",
            },
            {
              t: "Napuštanje bez vraćanja ga snižava",
              d: "To je jedino što ga snižava. Ako izgubiš vezu, imaš pun prozor za ponovno povezivanje, a vraćanje te ne košta ništa.",
            },
            {
              t: "Vreme ga popravlja",
              d: "Stara napuštanja blede. Loš niz od pre nekoliko meseci vredi vrlo malo prema mečevima koje si skorije završio.",
            },
          ],
        },
        {
          kind: "tiers",
          title: "Pet nivoa",
          items: [
            { tier: "exemplary", d: "skoro nikad ne odustaje." },
            { tier: "trusted", d: "je pouzdan partner." },
            { tier: "fair", d: "je uobičajen nivo." },
            { tier: "unreliable", d: "je zaključan za neke stolove." },
            { tier: "problematic", d: "je zaključan za većinu stolova." },
          ],
          text: "Domaćin sobe može da postavi minimum i da posebno odluči da li su dobrodošli igrači koji još nemaju skor.",
        },
        {
          kind: "note",
          text: "Dve stvari koje igrači najčešće pogrešno razumeju: predaja ne šteti tvojoj časti, i jedno loše veče te ne prati zauvek. Ponovno povezivanje posle prekida veze je uvek besplatno.",
        },
      ],
    },
  ],

  ui: {
    heroEyebrow: "Pravila · čitanje od 6 minuta",
    heroTitle: "Nauči Belu u jednom sedenju",
    heroIntro:
      "Bela je timska igra s kartama za četiri igrača sa špilom od 32 karte. Šest kratkih poglavlja u nastavku vode te od prve ruke sve do pobedničkog rezultata, sve što ti treba da se snađeš za stolom. Čitaj redom, ili skoči na ono što ti treba preko sadržaja levo.",
    facts: [
      { label: "Igrači", value: "4", caption: "dva tima po dvoje" },
      { label: "Špil", value: "32", caption: "od sedmice do Asa, četiri boje" },
      { label: "Karte po ruci", value: "8", caption: "osam u ruci pre prvog štiha" },
      { label: "Trka do", value: "1001", caption: "poena za pobedu" },
    ],
    tocTitle: "Sadržaj",
    footerTitle: "Spreman za prvu ruku?",
    footerBody:
      "Ovaj vodič prati te i u igru. Tokom ruke, pritisni dugme sa upitnikom u donjem desnom uglu i istih šest poglavlja se otvara, bez pauziranja igre.",
    footerCta: "Igraj",
    noteLabel: "Napomena",
    variantLabel: "Varijanta",
    diffLabel: "Razlikuje se u drugoj varijanti",
    pts: "poena",
    ladderTrumpTitle: "U adutskoj boji",
    ladderTrumpEyebrow: "Adut",
    ladderPlainTitle: "U svakoj drugoj boji",
    ladderPlainEyebrow: "Van aduta",
    colCard: "Karta",
    colPoints: "Poeni",
    colPower: "Snaga",
    meldKinds: { belot: "Par u adutu", set: "Kare", run: "Niz" },
    ovReference: "Uputstvo",
    ovTitle: "Pravila Bele",
    ovChapters: "Poglavlja",
    ovFullRef: "Potpuno uputstvo:",
    ovClose: "Zatvori",
  },
};
