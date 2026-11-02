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

  declarations: {
    belot: { name: "Bela", summary: "Kralj i Dama u adutu" },
    terca: { name: "Terca", summary: "Tri u nizu" },
    kvarta: { name: "Kvarta", summary: "Četiri u nizu" },
    kvinta: { name: "Kvinta", summary: "Pet ili više u nizu" },
    carre: { name: "Četiri iste", summary: "Četiri asa, kralja, dame ili desetke" },
    carre9: { name: "Četiri devetke" },
    carreJ: { name: "Četiri dečka" },
    bela: { name: "Bela", summary: "Svih osam aduta: meč je tvoj" },
  },

  sections: [
    {
      id: "deal",
      label: "Dijeljenje",
      title: "Dijeljenje i adut",
      lede: "Partneri sjede jedan nasuprot drugome. Igra ide udesno.",
      blocks: [
        {
          kind: "steps",
          items: [
            {
              variant: "bitola",
              otherVariantNote:
                "Hrvatska: dijeli se svih osam karata, posljednje dvije licem prema dolje.",
              t: "Po pet karata",
              d: "Tri, pa dvije. Sljedeća karta se okreće: kandidat za adut.",
            },
            {
              variant: "croatia",
              otherVariantNote: "Bitola: po pet karata i jedan okrenut kandidat za adut.",
              t: "Po osam karata",
              d: "Tri, tri, pa dvije licem prema dolje. Licitiraš na šest koje vidiš.",
            },
            {
              variant: "bitola",
              otherVariantNote: "Hrvatska: jedan krug, i zoveš bilo koju boju.",
              t: "Uzmi ili dalje",
              d: "Tko uzme okrenutu kartu, njezina boja je adut. Zatim se svima dijeli do osam.",
            },
            {
              variant: "croatia",
              otherVariantNote:
                "Bitola: prvi krug nudi okrenutu kartu, drugi bilo koju drugu boju.",
              t: "Zovi boju ili dalje",
              d: "Samo jedan krug. Propuštanje je „dalje“.",
            },
            {
              variant: "bitola",
              otherVariantNote: "Hrvatska: nema drugog kruga. Djelitelj mora zvati.",
              t: "Drugi krug",
              d: "Svi su rekli dalje? Zovi bilo koju drugu boju ili opet dalje.",
            },
            {
              variant: "croatia",
              otherVariantNote: "Bitola: drugi krug, pa novo miješanje.",
              t: "Djelitelj mora zvati",
              d: "Ako ostala trojica kažu dalje, djelitelj zove adut: „pod mus“.",
            },
            {
              variant: "bitola",
              otherVariantNote: "Hrvatska: svako dijeljenje se igra.",
              t: "Nitko nije uzeo",
              d: "Karte se miješaju i dijeli sljedeći igrač.",
            },
            {
              variant: "croatia",
              otherVariantNote: "Bitola: ništa se ne dijeli licem prema dolje.",
              t: "Otkrij posljednje dvije",
              d: "Kad se adut odredi, vidiš svoje dvije skrivene karte.",
            },
          ],
        },
      ],
    },
    {
      id: "cards",
      label: "Karte",
      title: "Vrijednost karata",
      lede: "U adutu Dečko i Devetka skaču na vrh.",
      blocks: [{ kind: "cards" }],
    },
    {
      id: "play",
      label: "Igra",
      title: "Igranje štiha",
      lede: "Osam štihova po ruci. Tko uzme štih, igra prvi u sljedećem.",
      blocks: [
        {
          kind: "list",
          numbered: true,
          items: [
            {
              t: "Prati boju",
              d: "Igraj boju koja je izašla ako je imaš, i nadmaši najjaču kartu te boje, osim ako je adut već rezao.",
            },
            {
              t: "Nemaš? Reži",
              d: "Igraj adut, jači od svakog aduta na stolu ako možeš.",
            },
            { t: "Nemaš ni adut? Bilo što", d: "Može bilo koja karta." },
          ],
        },
      ],
    },
    {
      id: "melds",
      label: "Zvanja",
      title: "Zvanja",
      lede: "Kombinacije u ruci donose bodove uz štihove.",
      blocks: [
        { kind: "melds" },
        {
          kind: "p",
          text: "Boduje samo tim s najjačim zvanjem, i to sva svoja zvanja.",
          help: "Isti bodovi: četiri iste pobjeđuju niz, zatim duži niz, viša najjača karta, niz u adutu i na kraju igrač koji je prvi na redu.",
        },
        {
          kind: "list",
          items: [
            {
              t: "Bela se uvijek računa",
              d: "Zovi je kad igraš Kralja ili Damu u adutu. Ako je preskočiš, nema je.",
            },
            {
              variant: "bitola",
              otherVariantNote: "Hrvatska: svi zovu odjednom, prije prvog štiha.",
              t: "Zovi uz prvu kartu",
              d: "Karte pokaži na početku drugog štiha.",
            },
            {
              variant: "croatia",
              otherVariantNote: "Bitola: zoveš kad igraš prvu kartu.",
              t: "Zovi prije prvog štiha",
              d: "Svi zovu u isto vrijeme; sva zvanja se pokažu zajedno.",
            },
            {
              variant: "bitola",
              otherVariantNote: "Hrvatska: karta se može računati u više zvanja.",
              t: "Jedna karta, jedno zvanje",
              d: "Dvije kombinacije dijele kartu? Računa se jača.",
            },
            {
              variant: "croatia",
              otherVariantNote: "Bitola: karta se računa samo u jednom zvanju.",
              t: "Karta se računa dvaput",
              d: "Kombinacije koje dijele kartu sve se boduju.",
            },
          ],
        },
      ],
    },
    {
      id: "scoring",
      label: "Bodovanje",
      title: "Bodovanje",
      lede: "Svaka ruka se dodaje na zajednički rezultat tima.",
      blocks: [
        { kind: "sum" },
        {
          kind: "list",
          items: [
            {
              t: "Tko zove adut mora proći",
              d: "Tim koji je zvao adut mora imati više bodova. Isto ili manje je pad: drugi tim uzima cijelu ruku.",
            },
            {
              t: "Štiglja",
              d: "Svih osam štihova: 100 umjesto 10 za posljednji, i cijela ruka.",
            },
            {
              t: "Pobjeda u meču",
              d: "Prvi preko 1001 nakon ruke. Oba preko? Pobjeđuje veći zbroj; neriješeno ide timu koji je zvao adut.",
            },
          ],
        },
      ],
    },
    {
      id: "options",
      label: "Kućna pravila",
      title: "Kućna pravila",
      lede: "Izmjene koje domaćin može odabrati kad stvara sobu.",
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
      { value: "1001", label: "za pobjedu" },
    ],
    chaptersLabel: "Poglavlja",
    variantLabel: "Varijanta",
    diffLabel: "Razlikuje se u drugoj varijanti",
    helpLabel: "Više",
    cardsTrump: "Adut",
    cardsOther: "Ostale boje",
    cardsFoot: "Najjača prva. Sve karte zajedno nose 152.",
    meldWin: "Pobjeda",
    sumCards: "karte",
    sumLastTrick: "posljednji štih",
    sumDeclarations: "zvanja",
    optionsFoot: "Standard je prvi. Kartica sobe označava svaku promjenu mjedenom bojom.",
    footerTitle: "Spreman za igru?",
    footerBody: "U meču dodirni ? da otvoriš ova pravila.",
    footerCta: "Igraj",
    ovTitle: "Pravila Bele",
    ovClose: "Zatvori",
  },
};
