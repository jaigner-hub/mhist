#!/usr/bin/env python3
"""
Autonomous Mode Safety Hook (PreToolUse)

Validates every Bash command against safety rules for unattended operation.
Defense-in-depth: settings.local.json deny rules catch obvious cases first,
this hook catches reordered flags, piped commands, and docker exec edge cases.

Usage: Registered as PreToolUse hook for Bash commands in settings.local.json
Input: JSON on stdin with tool_input.command
Output: JSON with decision ("block"/"allow") and reason, or exit(0) to allow silently
"""
import json
import re
import sys


def block(reason: str) -> None:
    """Print block decision and exit."""
    print(json.dumps({"decision": "block", "reason": reason}))
    sys.exit(0)


def check_git(command: str) -> None:
    """Block destructive git operations. Allow: add, commit, status, diff, log, branch (list)."""
    if not re.search(r"\bgit\b", command):
        return

    # Block git push (any form, including flags before 'push')
    if re.search(r"\bgit\b.*\bpush\b", command):
        block("Autonomous mode: git push is blocked. Commit locally only.")

    # Block git reset --hard
    if re.search(r"\bgit\b.*\breset\b.*--hard", command):
        block("Autonomous mode: git reset --hard is blocked (destructive).")

    # Block git clean -f/-fd/-fx
    if re.search(r"\bgit\b.*\bclean\b.*-[a-z]*f", command):
        block("Autonomous mode: git clean -f is blocked (deletes untracked files).")

    # Block git checkout . (discard all changes)
    if re.search(r"\bgit\b.*\bcheckout\b\s+\.", command):
        block("Autonomous mode: git checkout . is blocked (discards changes).")

    # Block git restore . (discard all changes)
    if re.search(r"\bgit\b.*\brestore\b\s+\.", command):
        block("Autonomous mode: git restore . is blocked (discards changes).")

    # Block git branch -D (force delete)
    if re.search(r"\bgit\b.*\bbranch\b.*-[a-zA-Z]*D", command):
        block("Autonomous mode: git branch -D is blocked (force-deletes branch).")

    # Block git stash drop/clear
    if re.search(r"\bgit\b.*\bstash\b\s+(drop|clear)", command):
        block("Autonomous mode: git stash drop/clear is blocked.")

    # Block interactive rebase
    if re.search(r"\bgit\b.*\brebase\b.*-[a-z]*i", command):
        block("Autonomous mode: interactive git rebase is blocked.")

    # Block force push flags anywhere
    if re.search(r"\bgit\b.*--force\b", command) or re.search(r"\bgit\b.*\s-f\b", command):
        # -f after git could be many things, only block near push
        if re.search(r"\bgit\b.*\bpush\b", command):
            block("Autonomous mode: force push is blocked.")


def check_remote_access(command: str) -> None:
    """Block SSH, SCP, and rsync to remote hosts."""
    if re.search(r"\bssh\b", command):
        block("Autonomous mode: ssh is blocked (no remote access).")

    if re.search(r"\bscp\b", command):
        block("Autonomous mode: scp is blocked (no remote access).")

    # rsync with : indicates remote target
    if re.search(r"\brsync\b", command) and ":" in command:
        block("Autonomous mode: rsync to remote hosts is blocked.")


def check_deployment(command: str) -> None:
    """Block deployment commands."""
    if re.search(r"\./deploy\.sh\b", command) or re.search(r"\bdeploy\.sh\b", command):
        block("Autonomous mode: deploy.sh is blocked.")

    if re.search(r"\bdeploy\s+(staging|production|prod|all)\b", command):
        block("Autonomous mode: deployment commands are blocked.")


def check_file_deletion(command: str) -> None:
    """Block rm, rmdir, unlink, shred."""
    # rm (any form)
    if re.search(r"\brm\s", command) or re.search(r"\brm$", command):
        block("Autonomous mode: rm is blocked (no file deletion).")

    if re.search(r"\brmdir\b", command):
        block("Autonomous mode: rmdir is blocked (no directory deletion).")

    if re.search(r"\bunlink\b", command):
        block("Autonomous mode: unlink is blocked (no file deletion).")

    if re.search(r"\bshred\b", command):
        block("Autonomous mode: shred is blocked (no file deletion).")


def check_privilege_escalation(command: str) -> None:
    """Block sudo, su, doas."""
    if re.search(r"\bsudo\b", command):
        block("Autonomous mode: sudo is blocked (no privilege escalation).")

    # su as standalone command (not substring like 'surplus')
    if re.search(r"\bsu\s", command) or re.search(r"^su$", command):
        block("Autonomous mode: su is blocked (no privilege escalation).")

    if re.search(r"\bdoas\b", command):
        block("Autonomous mode: doas is blocked (no privilege escalation).")


def check_network(command: str) -> None:
    """Block curl, wget, nc, ncat — but allow when inside pip/npm install."""
    # Skip check if the whole command is a pip/npm/yarn/pnpm install
    if re.search(r"\b(pip|pip3|npm|yarn|pnpm)\b", command):
        return

    if re.search(r"\bcurl\b", command):
        block("Autonomous mode: curl is blocked (use pip/npm for packages).")

    if re.search(r"\bwget\b", command):
        block("Autonomous mode: wget is blocked (use pip/npm for packages).")

    if re.search(r"\bncat\b", command) or re.search(r"\bnc\b", command):
        block("Autonomous mode: nc/ncat is blocked (no raw network access).")


def check_docker(command: str) -> None:
    """Validate docker exec commands. Block destructive SQL/management, allow safe operations."""
    # Block dangerous docker lifecycle commands
    if re.search(r"\bdocker\s+rm\b", command):
        block("Autonomous mode: docker rm is blocked.")

    if re.search(r"\bdocker\s+stop\b", command):
        block("Autonomous mode: docker stop is blocked.")

    if re.search(r"\bdocker\s+kill\b", command):
        block("Autonomous mode: docker kill is blocked.")

    if re.search(r"\bdocker-compose\s+down\b", command) or re.search(r"\bdocker\s+compose\s+down\b", command):
        block("Autonomous mode: docker-compose down is blocked.")

    # Only scrutinize docker exec further
    if not re.search(r"\bdocker\s+exec\b", command):
        return

    # Extract the part after docker exec ... (the actual command being run)
    # This handles: docker exec <container> bash -c "..."
    cmd_lower = command.lower()

    # Check for destructive SQL
    if re.search(r"\b(drop|delete|truncate)\b", cmd_lower):
        # Allow if it's clearly in a Python/code context (e.g., variable names)
        # But block if it looks like SQL
        if re.search(r"\b(drop\s+table|drop\s+database|drop\s+index)\b", cmd_lower):
            block("Autonomous mode: DROP TABLE/DATABASE/INDEX via docker exec is blocked.")
        if re.search(r"\bdelete\s+from\b", cmd_lower):
            block("Autonomous mode: DELETE FROM via docker exec is blocked.")
        if re.search(r"\btruncate\s+(table\s+)?\w", cmd_lower):
            block("Autonomous mode: TRUNCATE via docker exec is blocked.")

    if re.search(r"\balter\s+table\b.*\bdrop\b", cmd_lower):
        block("Autonomous mode: ALTER TABLE ... DROP via docker exec is blocked.")

    # Block destructive management commands
    if re.search(r"manage\.py\s+flush\b", cmd_lower):
        block("Autonomous mode: manage.py flush is blocked (destroys all data).")

    if re.search(r"manage\.py\s+reset_db\b", cmd_lower):
        block("Autonomous mode: manage.py reset_db is blocked.")

    if re.search(r"manage\.py\s+dbshell\b", cmd_lower):
        block("Autonomous mode: manage.py dbshell is blocked (interactive).")


def main():
    try:
        input_data = json.load(sys.stdin)
        tool_name = input_data.get("tool_name", "")

        # Only validate Bash commands
        if tool_name != "Bash":
            sys.exit(0)

        command = input_data.get("tool_input", {}).get("command", "")
        if not command:
            sys.exit(0)

        # Run all checks (each calls block() and exits if rule matches)
        check_git(command)
        check_remote_access(command)
        check_deployment(command)
        check_file_deletion(command)
        check_privilege_escalation(command)
        check_network(command)
        check_docker(command)

        # All checks passed — allow silently
        sys.exit(0)

    except Exception:
        # Never block on hook errors — fail open
        sys.exit(0)


if __name__ == "__main__":
    main()
