"""Loopback TLS SMTP receiver for integration tests; never relays mail."""
import base64,email,os,pathlib,re,socketserver,ssl,subprocess,threading,time
from email import policy

class Mailbox:
    username='mailer@example.test'
    password='fixture-smtp-authorization'
    def __init__(self,directory):
        self.messages=[];self.condition=threading.Condition();self.reject=False
        directory=pathlib.Path(directory);directory.mkdir(parents=True,exist_ok=True)
        self.cert=directory/'smtp-cert.pem';key=directory/'smtp-key.pem';config=directory/'smtp-openssl.cnf'
        config.write_text('[req]\ndistinguished_name=dn\nx509_extensions=ext\nprompt=no\n[dn]\nCN=localhost\n[ext]\nsubjectAltName=DNS:localhost,IP:127.0.0.1\nbasicConstraints=critical,CA:TRUE\nkeyUsage=critical,digitalSignature,keyEncipherment,keyCertSign\n')
        subprocess.run(['openssl','req','-x509','-newkey','rsa:2048','-nodes','-days','2','-config',str(config),'-keyout',str(key),'-out',str(self.cert)],check=True,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
        os.chmod(key,0o600)
        tls=ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER);tls.load_cert_chain(str(self.cert),str(key))
        owner=self
        class Handler(socketserver.StreamRequestHandler):
            def setup(self):
                self.request=tls.wrap_socket(self.request,server_side=True);self.request.settimeout(15);super().setup()
            def say(self,line):self.wfile.write((line+'\r\n').encode());self.wfile.flush()
            def handle(self):
                self.say('220 localhost fixture');authenticated=False;recipient=None
                while True:
                    raw=self.rfile.readline(8192)
                    if not raw:return
                    command=raw.decode('ascii','replace').strip();verb=command.split(' ',1)[0].upper()
                    if verb=='EHLO':self.say('250-localhost\r\n250-AUTH PLAIN\r\n250 8BITMIME')
                    elif verb=='AUTH':
                        try:
                            supplied=base64.b64decode(command.split(' ',2)[2]).split(b'\0')
                            authenticated=supplied[-2:]==[owner.username.encode(),owner.password.encode()]
                        except Exception:authenticated=False
                        self.say('235 authenticated' if authenticated else '535 rejected')
                    elif not authenticated:self.say('530 authentication required')
                    elif verb=='MAIL':self.say('250 sender accepted')
                    elif verb=='RCPT':recipient=command.split('<',1)[-1].split('>',1)[0];self.say('250 recipient accepted')
                    elif verb=='DATA':
                        self.say('354 start message');body=bytearray()
                        while True:
                            line=self.rfile.readline(65536)
                            if not line:return
                            if line==b'.\r\n':break
                            if line.startswith(b'..'):line=line[1:]
                            body.extend(line)
                            if len(body)>65536:self.say('552 too large');return
                        if owner.reject:self.say('451 test delivery rejected');continue
                        message=email.message_from_bytes(bytes(body),policy=policy.default);text=message.get_content()
                        code=re.search(r'(?<!\d)(\d{6})(?!\d)',text)
                        if not recipient or not code:self.say('554 invalid test mail');continue
                        with owner.condition:
                            owner.messages.append({'email':recipient,'code':code.group(1),'subject':str(message['Subject']),'text':text});owner.condition.notify_all()
                        self.say('250 accepted')
                    elif verb=='QUIT':self.say('221 bye');return
                    elif verb=='RSET':recipient=None;self.say('250 reset')
                    else:self.say('502 unsupported')
        class Server(socketserver.ThreadingTCPServer):
            allow_reuse_address=True;daemon_threads=True
            def handle_error(self,*args):pass
        self.server=Server(('127.0.0.1',0),Handler);self.thread=threading.Thread(target=self.server.serve_forever,daemon=True);self.thread.start()
    def environment(self):
        return {'SMTP_HOST':'127.0.0.1','SMTP_PORT':str(self.server.server_address[1]),'SMTP_USERNAME':self.username,'SMTP_AUTH_CODE':self.password,'MAIL_FROM':self.username,'SMTP_CA_FILE':str(self.cert),'SMTP_TIMEOUT_SECONDS':'10','ALLOW_LOCAL_PROVIDERS':'true'}
    def wait(self,recipient,after=0,timeout=5):
        deadline=time.monotonic()+timeout
        with self.condition:
            while True:
                for message in self.messages[after:]:
                    if message['email']==recipient:return message
                remaining=deadline-time.monotonic()
                if remaining<=0:raise AssertionError('local SMTP fixture did not receive expected message')
                self.condition.wait(remaining)
    def close(self):self.server.shutdown();self.server.server_close();self.thread.join(timeout=2)
