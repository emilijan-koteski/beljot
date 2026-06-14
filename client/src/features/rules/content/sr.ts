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
      summary: "Svih osam karata jedne boje, u jednoj ruci.",
      detail:
        "Najređa ruka u igri. Svih osam karata jedne boje kod jednog igrača. Odmah nosi ceo meč: taj tim dobija punih 1001 poen i igra staje istog trena kad se pokaže.",
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
        {
          kind: "steps",
          items: [
            {
              t: "Sedni na svoje mesto",
              d: "Sediš tačno naspram svog partnera; dvojica protivnika zauzimaju stolice s obe strane. Igra se kreće udesno oko stola.",
            },
            {
              t: "Sastavi špil",
              d: "Beljot se igra sa 32 karte. Uzmi običan špil i izbaci sve od 2 do 6. Ono što ostaje su sedmica, osmica, devetka, desetka, Žandar, Dama, Kralj i As u sve četiri boje. Time igraš.",
            },
            {
              t: "Podeli prvih pet",
              d: "Delilac obilazi dvaput, po tri karte pa dve, pa svako počinje sa pet u ruci. Ostatak špila ostaje licem nadole na sredini.",
            },
            {
              t: "Otvori adut",
              d: "Delilac okreće sledeću kartu sa špila licem nagore. Redom, svaki igrač može da je uzme, čime njena boja postaje adut za tu ruku, ili da preskoči. Čim je neko uzme, ta boja je adut i delilac deli ostatak karata dok svako ne drži osam. Adut pobeđuje sve iz druge tri boje, bez obzira na rang.",
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
      lede: "Imaj pravu kombinaciju u podeljenoj ruci i ona nosi poene sama po sebi. Zoveš je na svom redu u prvom štihu, pa je otkrivaš na početku drugog.",
      blocks: [
        {
          kind: "p",
          text: "Čim su karte podeljene i adut određen, proveri ruku za zvanja: nizove karata u nizu u jednoj boji, četiri iste, i par Kralj-i-Dama u adutu. Dama je Bela, Kralj je Rebela. Zvanje se radi na tvom redu u prvom štihu, dok igraš kartu, a zatim slažeš karte licem nagore za sve na početku drugog štiha. Bela i Rebela su izuzetak. Svaku zoveš kad igraš tu kartu tokom igre.",
        },
        { kind: "melds" },
        {
          kind: "rule",
          title: "Samo jedan tim je plaćen za zvanja",
          text: "Svaka strana ističe svoje jedino najbolje zvanje. Čije je jače, skuplja sva zvanja iz obe ruke tima, a drugi tim ne dobija ništa za svoja. Jednako vrede? Kare pobeđuje niz, pa kare asova (100) pobeđuje kvintu (100). Duži niz pobeđuje kraći, ali samo do kvinte. Kad obe strane imaju kvintu (pet ili više karata), dužina više ne znači ništa, nego pobeđuje niz sa višom gornjom kartom, isto kao i kod dva niza iste dužine. Još uvek izjednačeno? Niz u adutu nosi. A ako nijedan niz nije adut, prednost ima onaj od dvojice igrača koji je pre na redu, počev zdesna od delioca. Bela i Rebela stoje van ovog takmičenja, ko ih zove, uvek ih boduje.",
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
          kind: "note",
          text: "Ruke se igraju dok bar jedan tim ne sedne na 1001 ili više na kraju ruke. Ako oba tima pređu granicu u istoj ruci, strana sa više ukupnih poena osvaja meč. Za kraći meč, soba se može podesiti i na trku do 501 poena, uz potpuno ista pravila.",
        },
      ],
    },
  ],

  ui: {
    heroEyebrow: "Pravila · čitanje od 6 minuta",
    heroTitle: "Nauči Beljot u jednom sedenju",
    heroIntro:
      "Beljot je timska igra s kartama za četiri igrača sa špilom od 32 karte. Šest kratkih poglavlja u nastavku vode te od prve ruke sve do pobedničkog rezultata, sve što ti treba da se snađeš za stolom. Čitaj redom, ili skoči na ono što ti treba preko sadržaja levo.",
    facts: [
      { label: "Igrači", value: "4", caption: "dva tima po dvoje" },
      { label: "Špil", value: "32", caption: "od sedmice do Asa, četiri boje" },
      { label: "Karte po ruci", value: "8", caption: "podeljene 3, pa 2, pa 3" },
      { label: "Trka do", value: "1001", caption: "poena za pobedu" },
    ],
    tocTitle: "Sadržaj",
    footerTitle: "Spreman za prvu ruku?",
    footerBody:
      "Ovaj vodič prati te i u igru. Tokom ruke, pritisni dugme sa upitnikom u donjem desnom uglu i istih šest poglavlja se otvara, bez pauziranja igre.",
    footerCta: "Igraj",
    noteLabel: "Napomena",
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
    ovTitle: "Pravila Beljota",
    ovChapters: "Poglavlja",
    ovFullRef: "Potpuno uputstvo:",
    ovClose: "Zatvori",
  },
};
