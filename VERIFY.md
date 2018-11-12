# Theo-agent VERIFY

## Use authorized key signature

If you store authorized key' signature along with the authorized key, `theo-agent` is able to verify it before returning it to sshd.   
This will guarantee you that no one, in any case, will be able to inject unsolicited authorized keys and consequently get access to your server.

### Setup

Follow [theo-cli SIGN](https://github.com/theoapp/theo-cli/blob/master/SIGN.md) guide to create private/public key. Then copy only the public key to the server where `theo-agent` will run.

### Configure

With the `-verify` flag on, `theo-agent` will use the public key indicated in the `/etc/theo-agent/config.yml` to verifiy each signature.

This is an example of `config.yml`:

```
url: https://keys.sample.com
token: XXXXXXXX
public_key: /etc/theo-agent/public.pem
```

`sshd_config` must include the `-verify` flag in `AuthorizedKeysCommand` :

```
[...]
AuthorizedKeysCommand theo-agent -verify
AuthorizedKeysCommandUser theo-user
[...]
```
