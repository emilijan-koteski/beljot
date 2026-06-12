import { describe, expect, it } from "vitest";

import type { ActionType, CardId, MatchState, Phase, Rank, Suit, Variant } from "./matchTypes";

describe("gameTypes", () => {
  describe("CardId", () => {
    it("generates all 32 valid card IDs", () => {
      const suits: Suit[] = ["S", "H", "D", "C"];
      const ranks: Rank[] = ["7", "8", "9", "T", "J", "Q", "K", "A"];

      const allCards: CardId[] = [];
      for (const rank of ranks) {
        for (const suit of suits) {
          allCards.push(`${rank}${suit}` as CardId);
        }
      }

      expect(allCards).toHaveLength(32);

      const unique = new Set(allCards);
      expect(unique.size).toBe(32);
    });
  });

  describe("Phase", () => {
    it("covers all 8 game phases", () => {
      const phases: Phase[] = [
        "dealing",
        "bidding",
        "playing",
        "trick_resolving",
        "hand_scoring",
        "match_end",
        "paused",
        "disconnected",
      ];

      expect(phases).toHaveLength(8);
    });
  });

  describe("Variant", () => {
    it("includes bitola variant", () => {
      const variant: Variant = "bitola";
      expect(variant).toBe("bitola");
    });
  });

  describe("ActionType", () => {
    it("covers all 10 action types", () => {
      const actions: ActionType[] = [
        "play_card",
        "pick_trump",
        "pass_trump",
        "declare",
        "skip_declare",
        "announce_belot",
        "decline_belot",
        "pause",
        "unpause",
        "owner_unpause",
      ];

      expect(actions).toHaveLength(10);
    });
  });

  describe("MatchState shape", () => {
    it("accepts a valid MatchState object", () => {
      const state: MatchState = {
        id: 1,
        roomId: 42,
        variant: "bitola",
        matchMode: "1001",
        phase: "bidding",
        handNumber: 1,
        dealerSeat: 0,
        trumpSuit: null,
        trumpCallerSeat: null,
        trumpCandidate: { rank: "7", suit: "H" },
        biddingRound: 1,
        biddingPassCount: 0,
        deck: [],
        activePlayerSeat: 1,
        trickNumber: 0,
        currentTrick: [],
        leadSuit: null,
        trickWinnerSeat: null,
        awaitingDeclaration: false,
        declarationsResolved: false,
        players: [
          {
            hand: [],
            seat: 0,
            userId: 10,
            username: "Alice",
            team: "teamA",
            declarations: [],
            connected: true,
            isBot: false,
          },
          {
            hand: [],
            seat: 1,
            userId: 20,
            username: "Bob",
            team: "teamB",
            declarations: [],
            connected: true,
            isBot: false,
          },
          {
            hand: [],
            seat: 2,
            userId: 30,
            username: "Carol",
            team: "teamA",
            declarations: [],
            connected: true,
            isBot: false,
          },
          {
            hand: [],
            seat: 3,
            userId: 40,
            username: "Dave",
            team: "teamB",
            declarations: [],
            connected: true,
            isBot: false,
          },
        ],
        teamScores: [0, 0],
        handPoints: [0, 0],
        declarationPoints: [0, 0],
        belotPoints: [0, 0],
        tricksWon: [0, 0],
        pendingBelotSeat: null,
        belotAnnounced: false,
        winnerTeam: null,
        lastHandResult: null,
        turnExpiresAt: null,
        timerDurationSec: 0,
        previousPhase: "" as const,
        pausedPlayers: [false, false, false, false] as [boolean, boolean, boolean, boolean],
        pauseUsed: [false, false, false, false] as [boolean, boolean, boolean, boolean],
        surrenderProposerSeat: null,
        surrenderUsed: [false, false, false, false] as [boolean, boolean, boolean, boolean],
        turnTimeRemaining: 0,
        ownerSeat: 0,
        disconnectedSeat: -1,
        reconnectExpiresAt: null,
        playerReconnectExpiresAt: [null, null, null, null] as [
          string | null,
          string | null,
          string | null,
          string | null,
        ],
      };

      expect(state.phase).toBe("bidding");
      expect(state.players).toHaveLength(4);
      expect(state.variant).toBe("bitola");
    });
  });
});
