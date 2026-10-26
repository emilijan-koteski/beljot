import { beforeEach, describe, expect, it, vi } from "vitest";

import { axiosClient } from "@/shared/api/axiosClient";
import { listInvitablePlayers } from "@/shared/api/rooms";

describe("rooms API request contract", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("listInvitablePlayers issues GET /rooms/:id/invitable-players with the search term", async () => {
    const get = vi.spyOn(axiosClient, "get").mockResolvedValue(undefined as never);

    await listInvitablePlayers(7, "mar");

    expect(get).toHaveBeenCalledTimes(1);
    const [url, config] = get.mock.calls[0]!;
    expect(url).toBe("/rooms/7/invitable-players");
    expect(config).toEqual({ params: { search: "mar" } });
  });

  it("listInvitablePlayers sends no search param for an empty term", async () => {
    const get = vi.spyOn(axiosClient, "get").mockResolvedValue(undefined as never);

    await listInvitablePlayers(7, "");

    expect(get).toHaveBeenCalledTimes(1);
    const [url, config] = get.mock.calls[0]!;
    expect(url).toBe("/rooms/7/invitable-players");
    expect(config).toStrictEqual({ params: undefined });
  });
});
