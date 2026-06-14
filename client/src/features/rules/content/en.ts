// Beljot · Rules content — English (source copy, cleaned for terminology consistency).
import type { RulesLangData } from "./types";

export const en: RulesLangData = {
  cardNames: {
    J: "Jack",
    "9": "9",
    A: "Ace",
    "10": "10",
    K: "King",
    Q: "Queen",
    "8": "8",
    "7": "7",
  },
  trumpNotes: { J: "Strongest in trump", "7": "Weakest" },
  plainNotes: { A: "Strongest off-trump", "7": "Weakest" },

  declarations: {
    belot: {
      name: "Belote-Rebelote",
      summary: "The King and Queen of trump, both sitting in one hand.",
      detail:
        "The Queen is the Belote, the King the Rebelote — announce each as you play it, and it carries +20 points for your team.",
    },
    terca: {
      name: "Tierce",
      summary: "Three cards in a row, all the same suit.",
      detail:
        "For declarations the order runs 7, 8, 9, 10, Jack, Queen, King, Ace — and there’s no wrapping from Ace back round to 7.",
    },
    kvarta: {
      name: "Quarte",
      summary: "Four cards in a row, all the same suit.",
      detail:
        "A quarte always beats any tierce the other team holds, no matter which suits are in play.",
    },
    kvinta: {
      name: "Quint",
      summary: "Five or more cards in a row, same suit.",
      detail:
        "A quint always beats any quarte the other team holds, no matter which suits are in play. Any run of five-plus in a single suit is worth 100.",
    },
    carre: {
      name: "Carré",
      summary: "All four of one rank — Tens, Queens, Kings or Aces only.",
      detail:
        "Four of a kind from one of these four ranks. Sets of Nines and Jacks pay more and are scored separately.",
    },
    carre9: {
      name: "Carré of 9s",
      summary: "All four Nines.",
      detail:
        "The 9 of trump is the second-strongest card in the deck — so a full set of Nines pays one and a half times a regular carré.",
    },
    carreJ: {
      name: "Carré of Jacks",
      summary: "All four Jacks.",
      detail:
        "The biggest single declaration in the game. Catching all four Jacks in your eight dealt cards is rare — most players see it only a handful of times in a whole season.",
    },
    bela: {
      name: "Belote",
      summary: "All eight cards of one suit, held by a single player.",
      detail:
        "The rarest hand in the game — every card of one suit, all eight, in one hand. It wins the whole match on the spot: that player's team takes the full 1001 points and play stops the instant it's shown.",
    },
  },

  sections: [
    {
      id: "goal",
      label: "The goal",
      title: "Race your team to 1001",
      lede: "You and your partner share one score. First side to 1001 takes the match.",
      blocks: [
        {
          kind: "p",
          text: "You’re sat across from your partner, the two of you against the pair on either side. You share a single running score, and nothing resets between hands — the points just keep stacking until someone crosses 1001. Most matches wrap up in 6 to 12 hands.",
        },
        {
          kind: "p",
          text: "There are two ways to score. Win tricks, and you pocket the points printed on every card you capture. Hold the right cards, and you can announce combinations — a run of four in one suit, say, or the King and Queen of trump together — for a bonus on top. Tricks are your steady income; declarations are the big swings that flip a whole match.",
        },
      ],
    },
    {
      id: "basics",
      label: "Getting dealt in",
      title: "Shuffle, deal, take trump",
      lede: "Four players, 32 cards, eight to a hand, and a quick round to settle which suit is trump.",
      blocks: [
        {
          kind: "steps",
          items: [
            {
              t: "Take your seat",
              d: "You sit directly across from your partner; your two opponents take the chairs on either side. Play moves to the right around the table.",
            },
            {
              t: "Build the deck",
              d: "Beljot uses 32 cards. Grab a standard deck and toss out everything from 2 to 6. What’s left — 7, 8, 9, 10, Jack, Queen, King and Ace in all four suits — is what you play with.",
            },
            {
              t: "Deal the first five",
              d: "The dealer goes around twice — three cards each, then two — so everyone starts with five in hand. The rest of the deck stays face-down in the middle as the talon.",
            },
            {
              t: "Turn up the trump",
              d: "The dealer flips the next card off the talon face-up. In turn, each player can take it — making its suit trump for the hand — or pass. Once someone takes it, that suit is trump and the dealer deals out the rest until everyone holds eight. Trump beats anything from the other three suits, whatever its rank.",
            },
          ],
        },
      ],
    },
    {
      id: "cards",
      label: "Card values",
      title: "Trump plays by its own rules",
      lede: "In trump, the Jack and 9 jump to the top. Everywhere else, it’s the order you already know.",
      blocks: [
        {
          kind: "p",
          text: "Every card does two jobs. Its strength decides who wins the trick; its point value gets added to your score if you capture it. The two don’t always line up — a card can be powerful and worth nothing, or weak and worth plenty.",
        },
        {
          kind: "p",
          text: "In the three plain suits, the pecking order is the familiar one: Ace on top, then 10, King, Queen, Jack, and down. But the second a suit becomes trump, two cards leap up the chart. The Jack of trump turns into the strongest card in the whole deck, with the 9 of trump right behind it. The Ace and 10 of trump slip down to third and fourth. Getting quick at flipping between these two orders is most of what separates new players from sharp ones.",
        },
        { kind: "cards" },
        {
          kind: "note",
          text: "Add up every card in the deck and you get 152 points. Win the last trick of the hand and you grab another 10 (the “last trick” bonus), so there’s 162 on the table each hand before any declarations land.",
        },
      ],
    },
    {
      id: "play",
      label: "Playing a trick",
      title: "When you can play what",
      lede: "You’re rarely free to chuck down whatever you like. Three short rules cover almost every turn.",
      blocks: [
        {
          kind: "p",
          text: "A trick is one card from each of the four players, played in turn. Whoever wins it scoops all four cards into their team’s pile and leads the next one. Eight tricks and the hand is done.",
        },
        {
          kind: "rule",
          title: "Follow the suit that was led — and top it if you can",
          text: "If a Heart is led, you must play a Heart whenever you hold one. And you can’t duck out: if you hold a Heart higher than the best one already on the table, you have to play it. Only when every Heart in your hand is lower may you let go of a low one.",
        },
        {
          kind: "rule",
          title: "Out of the suit? You must trump — over the top if you can",
          text: "Can’t follow suit but still holding trump? You’re obliged to trump in. And if a trump is already down, you must beat it with a higher one when you can; only if all your trumps are lower may you play a small trump. The highest trump on the table takes the trick.",
        },
        {
          kind: "rule",
          title: "Cut by a trump? Following suit still comes first",
          text: "Once the trick has been cut by a trump, you must still follow the led suit if you hold it — but any card of that suit will do, since the trump is already winning and your suit can no longer take the trick. You only reach for a trump when you have none of the led suit; and if someone before you has already cut in, you must overtrump theirs with a higher trump if you can, or play any trump if you can’t.",
        },
        {
          kind: "p",
          text: "No card of the led suit and no trump either? Play whatever you fancy. It can’t win the trick — it just gets swept up by whoever does.",
        },
      ],
    },
    {
      id: "melds",
      label: "Declarations",
      title: "Some hands carry points of their own",
      lede: "Land the right combination in your dealt hand and it scores on its own — announced on your turn in the first trick, then revealed at the start of the second.",
      blocks: [
        {
          kind: "p",
          text: "Once the cards are dealt and trump is set, check your hand for declarations: runs of cards in a row in one suit, four of a kind, and the King-and-Queen-of-trump pair — the Queen is the Belote, the King the Rebelote. You announce a declaration on your turn during the first trick, as you play your card — then lay the cards face-up for everyone at the start of the second trick. Belote and Rebelote are the odd ones out — you announce each as you play that card during the hand.",
        },
        { kind: "melds" },
        {
          kind: "rule",
          title: "Only one team gets paid for declarations",
          text: "Each side puts forward its single best declaration. Whoever’s is stronger scoops up every declaration across both their hands — the other team scores nothing for theirs. A longer run beats a shorter one. Same length? The higher top card wins. Still tied? A run in trump takes it. Belote and Rebelote sit outside this contest — whoever announces them always scores them.",
        },
      ],
    },
    {
      id: "scoring",
      label: "Scoring",
      title: "Counting up — and the catch",
      lede: "The taker makes a promise: finish ahead, or hand the opponents everything you earned that hand.",
      blocks: [
        {
          kind: "steps",
          items: [
            {
              t: "Count the cards you caught",
              d: "Each team flips over its won tricks and totals the points on the cards inside. Across both teams it always comes to exactly 152.",
            },
            {
              t: "Add the last-trick bonus",
              d: "Won the eighth and final trick? That’s another 10 points — table slang calls it “dix de der”. Now you’re at 162 for the cards alone.",
            },
            {
              t: "Add the declarations",
              d: "The side that won the declarations contest adds up every combination across both partners’ hands. Any Belote or Rebelote called during the hand goes on top, for whoever announced it.",
            },
          ],
        },
        {
          kind: "rule",
          title: "The taker has to come out ahead",
          text: "The team that took trump must finish with strictly more points than the other side, declarations on both sides included. Fall short — or even tie — and the hand is lost: everything you scored that hand, cards and declarations alike, goes to your opponents instead. Players call this “going down”, and one bad hand can wipe out a comfortable lead.",
        },
        {
          kind: "note",
          text: "Hands keep coming until at least one team is sitting on 1001 or more at the end of a hand. If both teams cross the line on the same hand, the side with more total points takes the match. For a quicker match, a room can be set up as a race to 501 instead — every other rule stays exactly the same.",
        },
      ],
    },
  ],

  ui: {
    heroEyebrow: "Rules · 6-minute read",
    heroTitle: "Learn Beljot in one sitting",
    heroIntro:
      "Beljot is a partnership card game for four players with a 32-card deck. The six short chapters below take you from the deal all the way to the winning score — everything you need to hold your own at the table. Read straight through, or jump to whatever you need with the contents on the left.",
    facts: [
      { label: "Players", value: "4", caption: "two teams of two" },
      { label: "Deck", value: "32", caption: "7 up to Ace, four suits" },
      { label: "Cards per hand", value: "8", caption: "dealt 3 and 2, then trump" },
      { label: "Race to", value: "1001", caption: "points to win" },
    ],
    tocTitle: "Table of contents",
    footerTitle: "Ready for your first hand?",
    footerBody:
      "This guide tags along into the game, too. Mid-hand, tap the question mark in the bottom-right corner and these same six chapters slide open — no need to pause the play.",
    footerCta: "Play",
    noteLabel: "Note",
    pts: "pts",
    ladderTrumpTitle: "In the trump suit",
    ladderTrumpEyebrow: "Trump · adut",
    ladderPlainTitle: "In every other suit",
    ladderPlainEyebrow: "Off-trump",
    colCard: "Card",
    colPoints: "Points",
    colPower: "Power",
    meldKinds: { belot: "Trump pair", set: "Carré", run: "Run" },
    ovReference: "Reference",
    ovTitle: "Beljot rules",
    ovChapters: "Chapters",
    ovFullRef: "Full reference:",
    ovClose: "Close",
  },
};
