"use strict";

const fs = require("fs");
const { execFileSync } = require("child_process");
const path = require("path");

const VERSION_SOURCE = path.resolve(
  process.cwd(),
  "internal/cli/update_check.go",
);

function runGit(args) {
  return execFileSync("git", args, {
    encoding: "utf8",
    stdio: ["ignore", "pipe", "pipe"],
  }).trim();
}

function parseVersion(value) {
  const match = /^v?(\d+)\.(\d+)\.(\d+)$/.exec(String(value).trim());
  if (!match) {
    return null;
  }

  return {
    major: Number(match[1]),
    minor: Number(match[2]),
    patch: Number(match[3]),
  };
}

function compareVersions(left, right) {
  return (
    left.major - right.major ||
    left.minor - right.minor ||
    left.patch - right.patch
  );
}

function configuredVersion() {
  const source = fs.readFileSync(VERSION_SOURCE, "utf8");
  const match = /\bappVersion\s*=\s*"(\d+\.\d+\.\d+)"/.exec(source);
  if (!match) {
    throw new Error(`Could not read appVersion from ${VERSION_SOURCE}.`);
  }
  return match[1];
}

function setWorkflowOutput(name, value) {
  if (!process.env.GITHUB_OUTPUT) {
    return;
  }
  fs.appendFileSync(process.env.GITHUB_OUTPUT, `${name}=${value}\n`, "utf8");
}

function versionTags() {
  runGit(["fetch", "--tags", "--force"]);
  const output = runGit(["tag", "--list", "v*.*.*"]);
  if (!output) {
    return [];
  }

  return output
    .split(/\r?\n/)
    .map((tag) => ({ tag, version: parseVersion(tag) }))
    .filter((entry) => entry.version)
    .sort((left, right) => compareVersions(left.version, right.version));
}

async function githubRequest(apiPath, options = {}) {
  const response = await fetch(`https://api.github.com${apiPath}`, {
    ...options,
    headers: {
      Accept: "application/vnd.github+json",
      Authorization: `Bearer ${process.env.GITHUB_TOKEN}`,
      "Content-Type": "application/json",
      "User-Agent": "mcquery-release-workflow",
      "X-GitHub-Api-Version": "2022-11-28",
      ...options.headers,
    },
  });

  if (response.status === 404) {
    return { found: false, payload: null };
  }
  if (!response.ok) {
    const detail = await response.text();
    throw new Error(
      `GitHub API ${options.method || "GET"} ${apiPath} failed with ${response.status}: ${detail}`,
    );
  }

  return {
    found: true,
    payload: await response.json().catch(() => ({})),
  };
}

async function main() {
  const repository = process.env.GITHUB_REPOSITORY;
  const commit = process.env.GITHUB_SHA;
  if (!process.env.GITHUB_TOKEN || !repository || !commit) {
    throw new Error(
      "GITHUB_TOKEN, GITHUB_REPOSITORY, and GITHUB_SHA are required.",
    );
  }

  const version = configuredVersion();
  const parsedVersion = parseVersion(version);
  const tagName = `v${version}`;
  setWorkflowOutput("tag", tagName);
  const tags = versionTags();
  const latest = tags.at(-1);

  if (latest && compareVersions(parsedVersion, latest.version) < 0) {
    throw new Error(
      `Configured version ${tagName} is older than latest tag ${latest.tag}.`,
    );
  }

  const encodedTag = encodeURIComponent(tagName);
  const existingRelease = await githubRequest(
    `/repos/${repository}/releases/tags/${encodedTag}`,
  );
  if (existingRelease.found) {
    console.log(`Release ${tagName} already exists. Nothing to publish.`);
    return;
  }

  const tagAlreadyExists = tags.some((entry) => entry.tag === tagName);
  await githubRequest(`/repos/${repository}/releases`, {
    method: "POST",
    body: JSON.stringify({
      tag_name: tagName,
      target_commitish: commit,
      name: tagName,
      generate_release_notes: true,
      draft: false,
      prerelease: false,
    }),
  });

  console.log(
    tagAlreadyExists
      ? `Created release for existing tag ${tagName}.`
      : `Created tag and release ${tagName} for ${commit.slice(0, 7)}.`,
  );
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
