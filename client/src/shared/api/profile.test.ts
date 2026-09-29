import type { AxiosProgressEvent, AxiosRequestConfig } from "axios";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { axiosClient } from "@/shared/api/axiosClient";
import { removeAvatar, uploadAvatar } from "@/shared/api/profile";

/**
 * THE REQUEST HALF OF THE AVATAR CONTRACT.
 *
 * Every component test mocks `@/shared/api/profile` outright, so nothing else
 * would notice if the upload lost its multipart override: axios would serialise
 * the FormData as JSON, the server would find no `avatar` part and answer
 * AVATAR_MISSING on every upload, and every suite would stay green. These spy
 * on the one axios seam and pin what goes on the wire against the Go route
 * (PUT/DELETE /users/:id/avatar, multipart field "avatar").
 */
describe("avatar API request contract", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("uploadAvatar PUTs the file as multipart field `avatar` with its own 60 s deadline", async () => {
    const put = vi.spyOn(axiosClient, "put").mockResolvedValue(undefined as never);
    const file = new File([new Uint8Array(16)], "me.jpg", { type: "image/jpeg" });

    await uploadAvatar(7, file);

    expect(put).toHaveBeenCalledTimes(1);
    const [url, body, config] = put.mock.calls[0]! as [string, FormData, AxiosRequestConfig];
    expect(url).toBe("/users/7/avatar");
    expect(body).toBeInstanceOf(FormData);
    expect(body.get("avatar")).toBe(file);
    expect(config.headers).toEqual({ "Content-Type": "multipart/form-data" });
    expect(config.timeout).toBe(60_000);
  });

  it("reports upload progress as a fraction clamped to 1", async () => {
    const put = vi.spyOn(axiosClient, "put").mockResolvedValue(undefined as never);
    const onProgress = vi.fn();

    await uploadAvatar(7, new File(["x"], "me.png", { type: "image/png" }), onProgress);

    const config = put.mock.calls[0]![2] as AxiosRequestConfig;
    const report = config.onUploadProgress!;
    report({ loaded: 50, total: 200 } as AxiosProgressEvent);
    // Multipart framing can push `loaded` past the file's own size.
    report({ loaded: 260, total: 200 } as AxiosProgressEvent);
    // No total (a chunked body) is no progress to report, not a division by zero.
    report({ loaded: 10, total: undefined } as AxiosProgressEvent);

    expect(onProgress.mock.calls).toEqual([[0.25], [1]]);
  });

  it("removeAvatar DELETEs /users/:id/avatar", async () => {
    const del = vi.spyOn(axiosClient, "delete").mockResolvedValue(undefined as never);

    await removeAvatar(7);

    expect(del).toHaveBeenCalledTimes(1);
    expect(del.mock.calls[0]![0]).toBe("/users/7/avatar");
  });
});
