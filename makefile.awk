function get_make_targets(makefile, targets) {
    cmd = MAKE " 2>/dev/null -R -prq -f " MAKEFILE " :";
    count = 0;
    in_target = 0;

    while (cmd | getline line) {
        if (line ~ /^# File/) {
            in_target = 1;
        } else if (line ~ /^# Finished Make data base/) {
            in_target = 0;
        } else if (in_target && match(line, /^([a-z0-9][^:]+)/, arr)) {
            if (arr[1] ~ /%/) {
                continue; # skip wildcard targets
            }
            targets[arr[1]] = 1;
        }
    }
    close(cmd);
}

function get_git_remote() {
    cmd = GIT " 2>/dev/null config --get remote.origin.url";
    cmd | getline git_remote;
    close(cmd);

    return git_remote;
}

function get_git_version_exact_match() {
    cmd = GIT " 2>/dev/null describe --exact-match --dirty";
    cmd | getline git_version;
    close(cmd);
    return git_version;
}

function get_git_version_nearest_tag() {
    cmd = GIT " 2>/dev/null describe --tags --abbrev=0 --dirty";
    cmd | getline git_version;
    close(cmd);
    return git_version;
}

function get_git_version_suffix() {
    cmd = GIT " 2>/dev/null log -n 1 --pretty='format:%cd-%h' --date='format:%Y%m%d%H%M%S' --abbrev=12"
    cmd | getline git_version_suffix;
    close(cmd);
    return git_version_suffix;
}

function get_git_version() {
    git_version = get_git_version_exact_match();

    if (git_version == "") {
        git_version = get_git_version_nearest_tag();
    }

    if (git_version == "" ) {
        git_version = "v0";
    }

    git_version_suffix = get_git_version_suffix();
    if (git_version_suffix == "") {
        git_version_suffix = "00000000000000-000000000000";
    }

    git_version = git_version "-" git_version_suffix;

    if (index(git_version, "-dirty") > 0) {
        sub("-dirty", "", git_version);
        git_version = git_version "-dirty";
    }

    return git_version;
    
}

function print_targets(targets) {
    n = 0;
    for (category in targets) {
        categories[++n] = category; 
    }

    asort(categories);
    last_category = "";

    for (i = 1; i <= n; i++) {
        category = categories[i];

        if (category != "") {
            if (category != last_category) {
                printf("\n");
            }
            printf("Available '%s' targets:\n", category);
        }
        last_category = category;

        delete names;
        m = 0;
        for (name in targets[category]) {
            names[++m] = name;
        }

        asort(names);

        for (k = 1; k <= m; k++) {
            name = names[k];
            description = targets[category][name];

            printf("  %-20s %s\n", name, description);
        }
    }
}

BEGIN {
    split("", KNOWN_TARGETS); # Initialize array
    get_make_targets(MAKEFILE, KNOWN_TARGETS);
    split("", TARGETS); # Initialize array

    GIT_REMOTE = get_git_remote();
    GIT_VERSION = get_git_version();
}

match($0, /^\s*.PHONY:\s*([a-z0-9][^:]+)\s+##\s+(.+)$/, arr) {
    parse_targets(arr[1], arr[2])
}

function parse_targets(target, description) {
    # try direct match
    if (target in KNOWN_TARGETS) {
        name = target;
        add_target(TARGETS, name, description);
        next;
    }

    # try pattern match
    for (known_target in KNOWN_TARGETS) {
        regex = target;
        gsub(/[\.\^\$\*\+\?\(\)\[\]\{\}\|\\]/, "\\\\&", regex);  # Escape special characters
        gsub(/%/, "(.*)", regex);  # Convert "%" to ".*"

        if (match(known_target, "^" regex "$", arr2)) {
            name = known_target;
            desc = description
            gsub(/%/, arr2[1], desc);
            add_target(TARGETS, name, desc);
        }
    }
}

function add_target(targets, name, description) {
    category = "";
    desc = description

    if (match(desc, /^\[([^\]]+)\]\s*(.*)$/, arr3)) {
        category = arr3[1];
        desc = arr3[2];
    }

    targets[category][name] = desc;
}

END {
    if (GIT_REMOTE != "" && match(GIT_REMOTE, /\/([^\/]+)\.git/, arr)) {
        printf "%s version %s\n\n", arr[1], GIT_VERSION;
    } else {
        print NAME "\n";
    }

    print "Usage: make [TARGET...]";
    print "\nAvailable targets:";
    print_targets(TARGETS);

    if (GIT_REMOTE != "") {
        print "\n" GIT_REMOTE;
    }
}