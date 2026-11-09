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

  declarations: {
    belot: { name: "Bela", summary: "Kralj i Dama u adutu" },
    terca: { name: "Terca", summary: "Tri u nizu" },
    kvarta: { name: "Kvarta", summary: "Četiri u nizu" },
    kvinta: { name: "Kvinta", summary: "Pet ili više u nizu" },
    carre: { name: "Kare", summary: "Četiri asa, kralja, dame ili desetke" },
    carre9: { name: "Kare devetki" },
    carreJ: { name: "Kare žandara" },
    bela: { name: "Bela", summary: "Svih osam aduta: meč je tvoj" },
  },

  sections: [
    {
      id: "deal",
      label: "Deljenje",
      title: "Deljenje i adut",
      lede: "Partneri sede jedan naspram drugog. Igra ide udesno.",
      blocks: [
        {
          kind: "steps",
          items: [
            {
              variant: "bitola",
              otherVariantNote: "Hrvatska: deli se svih osam karata, poslednje dve licem nadole.",
              t: "Po pet karata",
              d: "Tri, pa dve. Sledeća karta se okreće: kandidat za adut.",
            },
            {
              variant: "croatia",
              otherVariantNote: "Bitola: po pet karata i jedan okrenut kandidat za adut.",
              t: "Po osam karata",
              d: "Tri, tri, pa dve licem nadole. Licitiraš na šest koje vidiš.",
            },
            {
              variant: "bitola",
              otherVariantNote: "Hrvatska: jedan krug, i zoveš bilo koju boju.",
              t: "Uzmi ili dalje",
              d: "Ko uzme okrenutu kartu, njena boja je adut. Zatim se svima deli do osam.",
            },
            {
              variant: "croatia",
              otherVariantNote:
                "Bitola: prvi krug nudi okrenutu kartu, drugi bilo koju drugu boju.",
              t: "Zovi boju ili dalje",
              d: "Samo jedan krug. Preskakanje je „dalje“.",
            },
            {
              variant: "bitola",
              otherVariantNote: "Hrvatska: nema drugog kruga. Delilac mora da zove.",
              t: "Drugi krug",
              d: "Svi su rekli dalje? Zovi bilo koju drugu boju ili opet dalje.",
            },
            {
              variant: "croatia",
              otherVariantNote: "Bitola: drugi krug, pa novo mešanje.",
              t: "Delilac mora da zove",
              d: "Ako ostala trojica kažu dalje, delilac zove adut: „pod mus“.",
            },
            {
              variant: "bitola",
              otherVariantNote: "Hrvatska: svako deljenje se igra.",
              t: "Niko nije uzeo",
              d: "Karte se mešaju i deli sledeći igrač.",
            },
            {
              variant: "croatia",
              otherVariantNote: "Bitola: ništa se ne deli licem nadole.",
              t: "Otkrij poslednje dve",
              d: "Kad se adut odredi, vidiš svoje dve skrivene karte.",
            },
          ],
        },
      ],
    },
    {
      id: "cards",
      label: "Karte",
      title: "Vrednost karata",
      lede: "U adutu Žandar i Devetka skaču na vrh.",
      blocks: [{ kind: "cards" }],
    },
    {
      id: "play",
      label: "Igra",
      title: "Igranje štiha",
      lede: "Osam štihova po ruci. Ko uzme štih, igra prvi u sledećem.",
      blocks: [
        {
          kind: "list",
          numbered: true,
          items: [
            {
              t: "Prati boju",
              d: "Igraj boju koja je izašla ako je imaš, i nadbij najjaču kartu te boje, osim ako je adut već sekao.",
            },
            {
              t: "Nemaš? Seci",
              d: "Igraj adut, jači od svakog aduta na stolu ako možeš.",
            },
            { t: "Nemaš ni adut? Bilo šta", d: "Može bilo koja karta." },
          ],
        },
      ],
    },
    {
      id: "melds",
      label: "Zvanja",
      title: "Zvanja",
      lede: "Kombinacije u ruci donose poene uz štihove.",
      blocks: [
        { kind: "melds" },
        {
          kind: "p",
          text: "Boduje samo tim sa najjačim zvanjem, i to sva svoja zvanja.",
          help: "Isti poeni: kare pobeđuje niz, zatim duži niz, viša najjača karta, niz u adutu i na kraju igrač koji je prvi na redu.",
        },
        {
          kind: "list",
          items: [
            {
              t: "Bela se uvek računa",
              d: "Zovi je kad igraš Kralja ili Damu u adutu. Ako je preskočiš, nema je.",
            },
            {
              variant: "bitola",
              otherVariantNote: "Hrvatska: svi zovu odjednom, pre prvog štiha.",
              t: "Pre karte: zovi ili nastavi",
              d: "Do štiha određenog za sto, podrazumevano trećeg. Zoveš sve što imaš; odigrana karta se više ne računa. Ostali tada dobijaju jednu priliku, najbolje se odmah boduje i zvanja se završavaju za tu ruku.",
            },
            {
              variant: "croatia",
              otherVariantNote: "Bitola: zvanja pre karte, do štiha određenog za sto.",
              t: "Zovi pre prvog štiha",
              d: "Svi zovu u isto vreme; sva zvanja se pokažu zajedno.",
            },
            {
              variant: "bitola",
              otherVariantNote: "Hrvatska: karta se može računati u više zvanja.",
              t: "Jedna karta, jedno zvanje",
              d: "Dve kombinacije dele kartu? Računa se jača.",
            },
            {
              variant: "croatia",
              otherVariantNote: "Bitola: karta se računa samo u jednom zvanju.",
              t: "Karta se računa dvaput",
              d: "Kombinacije koje dele kartu sve se boduju.",
            },
          ],
        },
      ],
    },
    {
      id: "scoring",
      label: "Bodovanje",
      title: "Bodovanje",
      lede: "Svaka ruka se dodaje na zajednički skor tima.",
      blocks: [
        { kind: "sum" },
        {
          kind: "list",
          items: [
            {
              variant: "bitola",
              otherVariantNote: "Hrvatska: i nerešeno je pad.",
              t: "Ko zove adut mora da prođe",
              d: "Tim koji je zvao adut mora imati više poena. Manje je pad: drugi tim uzima celu ruku. Kod nerešenog (81:81) drugi tim zadržava svoju polovinu, a polovina onoga ko zove adut visi: u sledećoj ruci dobija je ko zove adut ako prođe, inače njegovi protivnici.",
            },
            {
              variant: "croatia",
              otherVariantNote:
                "Bitola: kod nerešenog polovina onoga ko zove adut visi do sledeće ruke.",
              t: "Ko zove adut mora da prođe",
              d: "Tim koji je zvao adut mora imati više poena. Isto ili manje je pad: drugi tim uzima celu ruku.",
            },
            {
              t: "Kapot",
              d: "Svih osam štihova: 100 umesto 10 za poslednji, i cela ruka.",
            },
            {
              t: "Pobeda u meču",
              d: "Prvi preko 1001 posle ruke. Oba preko? Pobeđuje veći zbir; nerešeno ide timu koji je zvao adut.",
            },
          ],
        },
      ],
    },
    {
      id: "options",
      label: "Kućna pravila",
      title: "Kućna pravila",
      lede: "Izmene koje domaćin može da izabere kad pravi sobu.",
      blocks: [{ kind: "options" }],
    },
  ],

  ui: {
    heroEyebrow: "Pravila",
    heroTitle: "Kako se igra Bela",
    heroIntro: "Dva tima po dvoje, 32 karte. Nosi štihove, zovi kombinacije, stigni prvi do 1001.",
    facts: [
      { value: "4", label: "igrača" },
      { value: "32", label: "karte" },
      { value: "8", label: "u ruci" },
      { value: "1001", label: "za pobedu" },
    ],
    chaptersLabel: "Poglavlja",
    variantLabel: "Varijanta",
    diffLabel: "Razlikuje se u drugoj varijanti",
    helpLabel: "Više",
    cardsTrump: "Adut",
    cardsOther: "Ostale boje",
    cardsFoot: "Najjača prva. Sve karte zajedno nose 152.",
    meldWin: "Pobeda",
    sumCards: "karte",
    sumLastTrick: "poslednji štih",
    sumDeclarations: "zvanja",
    optionsFoot: "Standard je prvi. Kartica sobe označava svaku promenu mesinganom bojom.",
    footerTitle: "Spreman za igru?",
    footerBody: "U meču dodirni ? da otvoriš ova pravila.",
    footerCta: "Igraj",
    ovTitle: "Pravila Bele",
    ovClose: "Zatvori",
  },
};
