import type { RulesLangData } from "./types";

export const en: RulesLangData = {
  cardNames: {
    J: "Jack",
    "9": "Nine",
    A: "Ace",
    "10": "Ten",
    K: "King",
    Q: "Queen",
    "8": "Eight",
    "7": "Seven",
  },

  declarations: {
    belot: { name: "Belote", summary: "King and Queen of trump" },
    terca: { name: "Tierce", summary: "Three in a row" },
    kvarta: { name: "Quarte", summary: "Four in a row" },
    kvinta: { name: "Quint", summary: "Five or more in a row" },
    carre: { name: "Carré", summary: "Four Aces, Kings, Queens or 10s" },
    carre9: { name: "Four Nines" },
    carreJ: { name: "Four Jacks" },
    bela: { name: "Belote", summary: "All eight trumps: the match is yours" },
  },

  sections: [
    {
      id: "deal",
      label: "Deal",
      title: "Deal and trump",
      lede: "Partners sit across. Play moves to the right.",
      blocks: [
        {
          kind: "steps",
          items: [
            {
              variant: "bitola",
              otherVariantNote: "Croatian: all eight cards are dealt, the last two face down.",
              t: "Five cards each",
              d: "Three, then two. The next card goes face up: the trump candidate.",
            },
            {
              variant: "croatia",
              otherVariantNote: "Bitola: five cards each and one face-up trump candidate.",
              t: "Eight cards each",
              d: "Three, three, then two face down. You bid on the six you can see.",
            },
            {
              variant: "bitola",
              otherVariantNote: "Croatian: one round, and you name any suit.",
              t: "Take it or pass",
              d: "Taking the face-up card makes its suit trump. Then everyone is dealt to eight.",
            },
            {
              variant: "croatia",
              otherVariantNote:
                "Bitola: round one offers the face-up card, round two any other suit.",
              t: "Name a suit or pass",
              d: "One round only. Passing is “dalje”.",
            },
            {
              variant: "bitola",
              otherVariantNote: "Croatian: no second round. The dealer must choose.",
              t: "Round two",
              d: "Everyone passed? Name any other suit, or pass again.",
            },
            {
              variant: "croatia",
              otherVariantNote: "Bitola: a second round, then a reshuffle.",
              t: "The dealer must choose",
              d: "If the other three pass, the dealer names trump: “pod mus”.",
            },
            {
              variant: "bitola",
              otherVariantNote: "Croatian: every deal is played.",
              t: "Nobody took it",
              d: "The cards are reshuffled and the next player deals.",
            },
            {
              variant: "croatia",
              otherVariantNote: "Bitola: nothing is dealt face down.",
              t: "Turn up your last two",
              d: "Once trump is set, your two hidden cards are yours to see.",
            },
          ],
        },
      ],
    },
    {
      id: "cards",
      label: "Cards",
      title: "Card values",
      lede: "In trump, the Jack and the 9 jump to the top.",
      blocks: [{ kind: "cards" }],
    },
    {
      id: "play",
      label: "Play",
      title: "Playing a trick",
      lede: "Eight tricks a hand. The winner of a trick leads the next.",
      blocks: [
        {
          kind: "list",
          numbered: true,
          items: [
            {
              t: "Follow suit",
              d: "Play the led suit if you can, and beat the best card of it unless a trump has already cut.",
            },
            {
              t: "Can’t? Trump",
              d: "Play a trump, higher than any trump on the table if you can.",
            },
            { t: "No trump either? Anything", d: "Any card is fine." },
          ],
        },
      ],
    },
    {
      id: "melds",
      label: "Declarations",
      title: "Declarations",
      lede: "Combinations in your hand score on top of your tricks.",
      blocks: [
        { kind: "melds" },
        {
          kind: "p",
          text: "Only the team with the best declaration scores, and then all of theirs.",
          help: "Equal points: four of a kind beats a run, then the longer run, the higher top card, a run in trump, and finally the player who comes first.",
        },
        {
          kind: "list",
          items: [
            {
              t: "Belote always counts",
              d: "Announce it as you play the King or Queen of trump. Skip it and it’s gone.",
            },
            {
              variant: "bitola",
              otherVariantNote: "Croatian: everyone declares at once, before the first trick.",
              t: "Before your card: declare or continue",
              d: "Up to the trick set for the table, the third by default. Declaring takes all you hold; a card already played no longer counts. The others then get one chance, the best scores at once, and declaring ends for the hand.",
            },
            {
              variant: "croatia",
              otherVariantNote:
                "Bitola: declare before your card, up to the trick set for the table.",
              t: "Declare before the first trick",
              d: "Everyone declares at the same time; all are shown together.",
            },
            {
              variant: "bitola",
              otherVariantNote: "Croatian: a card can count in several declarations.",
              t: "One card, one declaration",
              d: "Two combinations sharing a card? The better one counts.",
            },
            {
              variant: "croatia",
              otherVariantNote: "Bitola: a card counts in one declaration only.",
              t: "A card can count twice",
              d: "Combinations sharing a card all score.",
            },
          ],
        },
      ],
    },
    {
      id: "scoring",
      label: "Scoring",
      title: "Scoring",
      lede: "Each hand adds to one running team score.",
      blocks: [
        { kind: "sum" },
        {
          kind: "list",
          items: [
            {
              variant: "bitola",
              otherVariantNote: "Croatian: a tie is a fall too.",
              t: "The taker must win the hand",
              d: "The team that chose trump needs more points. Less is a fall: the other team takes the whole hand. On a tie (81:81) the other team keeps its half and the taker's half hangs: the next hand's taker gets it by winning that hand, otherwise their opponents do.",
            },
            {
              variant: "croatia",
              otherVariantNote: "Bitola: on a tie the taker's half hangs until the next hand.",
              t: "The taker must win the hand",
              d: "The team that chose trump needs more points. A tie or less is a fall: the other team takes the whole hand.",
            },
            {
              t: "Capot",
              d: "Win all eight tricks: 100 instead of 10 for the last one, and the whole hand.",
            },
            {
              t: "Winning the match",
              d: "First past 1001 after a hand. Both past it? The higher total wins; a tie goes to the taker.",
            },
          ],
        },
      ],
    },
    {
      id: "options",
      label: "House rules",
      title: "House rules",
      lede: "Tweaks a host can choose when creating a room.",
      blocks: [{ kind: "options" }],
    },
  ],

  ui: {
    heroEyebrow: "Rules",
    heroTitle: "How to play Belote",
    heroIntro: "Two teams of two, 32 cards. Win tricks, declare combinations, reach 1001 first.",
    facts: [
      { value: "4", label: "players" },
      { value: "32", label: "cards" },
      { value: "8", label: "in hand" },
      { value: "1001", label: "to win" },
    ],
    chaptersLabel: "Chapters",
    variantLabel: "Variant",
    diffLabel: "Differs in the other variant",
    helpLabel: "More",
    cardsTrump: "Trump",
    cardsOther: "Other suits",
    cardsFoot: "Strongest first. The cards add up to 152.",
    meldWin: "Win",
    sumCards: "cards",
    sumLastTrick: "last trick",
    sumDeclarations: "declarations",
    optionsFoot: "Standard first. The room card marks every change in brass.",
    footerTitle: "Ready to play?",
    footerBody: "In a match, tap ? to open these rules.",
    footerCta: "Play",
    ovTitle: "Belote rules",
    ovClose: "Close",
  },
};
