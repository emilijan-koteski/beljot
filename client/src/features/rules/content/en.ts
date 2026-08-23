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
      summary: "All eight cards of the trump suit, held by a single player.",
      detail:
        "The rarest hand in the game — every card of trump, all eight, in one hand. It wins the whole match on the spot: as soon as trump is settled and the hands are complete, that player's team is declared the winner, the match ends there, and not a single card is played.",
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
        // ONE block, not one per variant. The first two steps are word-for-word
        // identical in both rulesets and are authored once; the four after them
        // are scoped, each carrying its own note about that specific step's
        // counterpart. Bitola and Croatian items alternate so that filtering
        // either variant leaves a correctly ordered six-step sequence.
        {
          kind: "steps",
          items: [
            {
              t: "Take your seat",
              d: "You sit directly across from your partner; your two opponents take the chairs on either side. Play moves to the right around the table.",
            },
            {
              t: "Build the deck",
              d: "Belote uses 32 cards. Grab a standard deck and toss out everything from 2 to 6. What’s left — 7, 8, 9, 10, Jack, Queen, King and Ace in all four suits — is what you play with.",
            },
            {
              variant: "bitola",
              otherVariantNote:
                "Croatian rules deal all eight cards before anyone bids — three, three, then a last two face-down that stay hidden until trump is settled — and turn nothing face-up.",
              t: "Deal five each, then turn one up",
              d: "The dealer goes around twice — three cards each, then two — so everyone starts with five in hand. The next card off the deck goes face-up on the table as the trump candidate, and the eleven behind it stay face-down as the talon.",
            },
            {
              variant: "croatia",
              otherVariantNote:
                "Bitola rules stop the deal at five cards each and turn the next card face-up as the trump candidate, holding the other eleven back as the talon.",
              t: "Deal all eight up front",
              d: "The dealer goes around three times — three cards each, three more, then a last two dealt face-down. Every card is out before anyone bids: six you can look at, and two nobody can, not even you.",
            },
            {
              variant: "bitola",
              otherVariantNote:
                "In Croatian rules there is no card to take and only a single round of bidding: each player names any one of the four suits outright, judging by the six cards they can see, and draws no extra card.",
              t: "Round one: take that card, or pass",
              d: "Starting to the dealer’s right, each player either takes the face-up card — making its suit trump for the hand — or passes. Whoever takes it keeps it as one of their eight, and the dealer deals out the talon until every hand is full. Trump beats anything from the other three suits, whatever its rank.",
            },
            {
              variant: "croatia",
              otherVariantNote:
                "In Bitola rules round one offers that one face-up card: take it and its suit becomes trump, and the taker keeps the card as one of their eight.",
              t: "One round: name a suit, or say “dalje”",
              d: "Nothing is turned face-up and there is no candidate card to take. Starting to the dealer’s right, each player in turn either names any one of the four suits as trump — judging by the six cards they can see — or passes by saying “dalje”. Whoever names a suit draws no extra card; their hand is already dealt. Trump beats anything from the other three suits, whatever its rank.",
            },
            {
              variant: "bitola",
              otherVariantNote:
                "Croatian rules have no second round: three passes leave the choice with the dealer, who speaks last and must name a suit — “pod mus”.",
              t: "Round two: name a suit — but not that one",
              d: "All four passed? The same order goes round again, and this time you name a suit outright instead of taking the card. The candidate’s suit is spent and locked out, so you are choosing between the other three. The taker still picks the face-up card up with the rest of the deal.",
            },
            {
              variant: "croatia",
              otherVariantNote:
                "Bitola rules never force the dealer: if all four pass, a second round opens where any suit but the candidate’s can be named — and if that passes out too, the deal is reshuffled.",
              t: "The dealer speaks last — “pod mus”",
              d: "The dealer bids fourth and has no right to pass: if the other three have all said “dalje”, the dealer must name a suit, still seeing only their own six cards. That forced call is known as playing “pod mus”. There is no second round and no reshuffle in these rules — every deal gets played.",
            },
            {
              variant: "bitola",
              otherVariantNote:
                "Croatian rules never pass a deal out: the dealer speaks last in the single round and has to name a suit, so every deal gets played.",
              t: "Passed out twice? Fresh deal",
              d: "If round two passes out as well, nobody plays the hand: all 32 cards go back together and are shuffled, the deal moves to the next player on the right, and the whole thing starts over.",
            },
            {
              variant: "croatia",
              otherVariantNote:
                "Bitola rules deal nothing face-down, so there is nothing to reveal — the taker’s hand fills up from the talon instead, and a twice-passed-out deal is reshuffled.",
              t: "Your last two turn up",
              d: "Only once trump is named do the two face-down cards turn face-up — each player’s own pair, for that player alone; nobody sees anyone else’s. From there everyone holds all eight cards and the hand is played out.",
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
      lede: "Land the right combination in your dealt hand and it scores on its own, on top of whatever your tricks are worth.",
      blocks: [
        {
          kind: "p",
          text: "Once the cards are dealt and trump is set, check your hand for declarations: runs of cards in a row in one suit, four of a kind, and the King-and-Queen-of-trump pair — the Queen is the Belote, the King the Rebelote. Belote and Rebelote are the odd ones out — you announce each as you play that card, wherever in the hand it falls.",
        },
        {
          kind: "p",
          variant: "bitola",
          otherVariantNote:
            "Croatian rules give declaring a phase of its own between bidding and the first trick: every seat declares or skips, the whole table is revealed at once, and only then is a card played.",
          text: "There is no separate round for declaring. You announce yours on your turn during the first trick, as you play your card — then lay the cards face-up for everyone to see at the start of the second trick.",
        },
        {
          kind: "p",
          variant: "croatia",
          otherVariantNote:
            "Bitola rules have no separate phase: you declare on your turn during the first trick, as you play your card, and lay the cards face-up at the start of the second.",
          text: "Declaring gets a phase of its own, between bidding and the first trick. Each seat in turn declares or skips, the whole table’s declarations are then revealed together, and only after that does the first trick begin.",
        },
        { kind: "melds" },
        {
          kind: "rule",
          title: "Only one team gets paid for declarations",
          text: "Each side puts forward its single best declaration. Whoever’s is stronger scoops up every declaration across both their hands — the other team scores nothing for theirs. Worth the same? A four-of-a-kind beats a run — a carré of Aces (100) takes it over a quint (100). A longer run beats a shorter one — up to a quint. Once both runs are quints (five cards or more), length stops mattering: the higher top card wins, as it does between two runs of equal length. Still level? A run in trump takes it — and if neither is trump, the tie falls to whichever player comes first in the turn order, starting to the dealer’s right. Belote and Rebelote sit outside this contest — whoever announces them always scores them.",
        },
        {
          kind: "rule",
          variant: "bitola",
          otherVariantNote:
            "Croatian rules put no such limit on a card: it counts in every declaration it belongs to, so two combinations sharing a card both score in full.",
          title: "One card, one declaration",
          text: "A single card cannot be counted twice. If two of your combinations need the same card — a tierce in Hearts and a carré of Kings that both want your King of Hearts — only the more valuable one counts and the other is dropped. Worth the same? The four-of-a-kind is the one that stands.",
        },
        {
          kind: "rule",
          variant: "croatia",
          otherVariantNote:
            "Bitola rules count one card toward one declaration only: of two combinations sharing a card the weaker one is dropped, and at equal value the four-of-a-kind is the one kept.",
          title: "One card can count more than once",
          text: "The same card may belong to several declarations at once, and each of them scores in full. A tierce in Hearts and a carré of Kings both counting your King of Hearts is fine — you announce both and neither is discounted.",
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
          kind: "rule",
          title: "Take all eight tricks and it’s a capot",
          text: "Sweep the hand and the 10-point last-trick bonus is replaced by 100 — the last trick is yours anyway. Your team then takes every point on the table: all the card points, that bonus, and both teams’ declarations, the other side’s included. A team that wins no trick at all banks nothing, not even the declarations it had won.",
        },
        {
          kind: "note",
          text: "Hands keep coming until at least one team is sitting on 1001 or more at the end of a hand. If both teams cross the line on the same hand, the side with more total points takes the match — and if the two totals are exactly equal, it goes to the team that took trump. For a quicker match, a room can be set up as a race to 501 instead — every other rule stays exactly the same.",
        },
      ],
    },
    {
      id: "honour",
      label: "Honor",
      title: "Finishing what you start",
      lede: "Belote is a partnership game: a player who walks out mid-match ruins it for three other players. Honor shows how reliable a teammate you are.",
      blocks: [
        {
          kind: "p",
          text: "Your honor score is the share of matches you finish, weighted so recent play counts for far more than old history. A new player starts at 80 and shows no score at all until they have played 5 matches — there is nothing meaningful to measure before that.",
        },
        {
          kind: "steps",
          items: [
            {
              t: "Finishing a match lifts it",
              d: "Win or lose. Honor is not a measure of skill — it only asks whether you were still at the table at the end.",
            },
            {
              t: "Surrendering still counts as finished",
              d: "A surrendered match is a completed match. Agreeing to end it with your partner is the opposite of walking out, and it lifts your score exactly like any other finish.",
            },
            {
              t: "Dropping out and not returning lowers it",
              d: "This is the only thing that does. If you lose connection you have the full reconnect window to come back, and coming back costs you nothing.",
            },
            {
              t: "Time repairs it",
              d: "Old abandonments fade. A bad run months ago counts for very little against the matches you have finished more recently.",
            },
          ],
        },
        {
          kind: "tiers",
          title: "The five tiers",
          items: [
            { tier: "exemplary", d: "practically never quits." },
            { tier: "trusted", d: "is a reliable partner." },
            { tier: "fair", d: "is the usual range." },
            { tier: "unreliable", d: "is locked out of some tables." },
            { tier: "problematic", d: "is locked out of most tables." },
          ],
          text: "A room's host can require a minimum, and can separately decide whether players with no score yet are welcome.",
        },
        {
          kind: "note",
          text: "Two things players most often get wrong: surrendering does not hurt your honor, and a single bad night does not follow you around. Reconnecting after a dropped connection is always free.",
        },
      ],
    },
  ],

  ui: {
    heroEyebrow: "Rules · 6-minute read",
    heroTitle: "Learn Belote in one sitting",
    heroIntro:
      "Belote is a partnership card game for four players with a 32-card deck. The six short chapters below take you from the deal all the way to the winning score — everything you need to hold your own at the table. Read straight through, or jump to whatever you need with the contents on the left.",
    facts: [
      { label: "Players", value: "4", caption: "two teams of two" },
      { label: "Deck", value: "32", caption: "7 up to Ace, four suits" },
      { label: "Cards per hand", value: "8", caption: "eight in hand before trick one" },
      { label: "Race to", value: "1001", caption: "points to win" },
    ],
    tocTitle: "Table of contents",
    footerTitle: "Ready for your first hand?",
    footerBody:
      "This guide tags along into the game, too. Mid-hand, tap the question mark in the bottom-right corner and these same six chapters slide open — no need to pause the play.",
    footerCta: "Play",
    noteLabel: "Note",
    variantLabel: "Variant",
    diffLabel: "Differs in the other variant",
    pts: "pts",
    ladderTrumpTitle: "In the trump suit",
    ladderTrumpEyebrow: "Trump",
    ladderPlainTitle: "In every other suit",
    ladderPlainEyebrow: "Off-trump",
    colCard: "Card",
    colPoints: "Points",
    colPower: "Power",
    meldKinds: { belot: "Trump pair", set: "Carré", run: "Run" },
    ovReference: "Reference",
    ovTitle: "Belote rules",
    ovChapters: "Chapters",
    ovFullRef: "Full reference:",
    ovClose: "Close",
  },
};
