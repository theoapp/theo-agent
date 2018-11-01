# theo-agent
Theo agent

## Install
- [Linux install](#linux-install)

### Linux Install

1. Simply download one of the binaries for your system:

```
# Linux x86-64
sudo wget -O /usr/local/bin/theo-agent TBD/latest/binaries/theo-agent-linux-amd64

# Linux x86
sudo wget -O /usr/local/bin/theo-agent TBD/latest/binaries/theo-agent-linux-386

# Linux arm
sudo wget -O /usr/local/bin/theo-agent TBD/latest/binaries/theo-agent-linux-arm
```

2. Give it permissions to execute:

```
sudo chmod +x /usr/local/bin/theo-agent
```

3. Create a Theo Agent user:

```
sudo useradd --comment 'Theo Agent' --create-home theo-agent --shell /bin/false
```

4. Install 

    1. Full Automatic install

        ```
        sudo theo-agent -install -no-interactive -sshd-config -url ${THEO_URL} -token ${THEO_CLIENT_TOKEN}
        ```

    2. Semi-Automatic install

        ```
        sudo theo-agent -install -no-interactive -url ${THEO_URL} -token ${THEO_CLIENT_TOKEN}
        ```

        Edit `/etc/ssh/sshd_config` as suggested

    3. Semi-manual install

        ```
        sudo theo-agent -install
        ```

        Answer to the questions and edit `/etc/ssh/sshd_config` as suggested

    4. Manual install

        Create a `config.yml` file (default is */etc/theo-agent/config.yml*):

        ```
        url: THEO_URL
        token: THEO_CLIENT_TOKEN
        ```

        Create a cache directory (default is */var/cache/theo-agent*):

        ```
        mkdir /var/cache/theo-agent
        chmod 755 /var/cache/theo-agent
        ```

        Modify `/etc/ssh/sshd_config` (if you changed the default path, add the options to the command)

        ```
        PasswordAuthentication no
        AuthorizedKeysFile /var/cache/theo-agent/%u
        AuthorizedKeysCommand /usr/local/bin/theo-agent [-config-file /path/to/config.yml] [-cache-path /path/to/cache/dir]
        AuthorizedKeysCommandUser theo-agent
        ```