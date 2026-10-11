"""Exercise enforced admission with server dry runs and an ephemeral test identity."""
import argparse
import json
import subprocess


def main():
    parser=argparse.ArgumentParser()
    parser.add_argument('--kubeconfig',required=True)
    args=parser.parse_args()
    command=['kubectl','--kubeconfig='+args.kubeconfig]
    namespace='homelab-verification'
    identity='homelab-admission-test'
    created=[]
    def apply(document):
        subprocess.run(command+['apply','-f','-'],input=json.dumps(document),text=True,check=True,capture_output=True)
        created.append(document)
    def reject(document, expected, as_identity=False):
        impersonation=['--as=system:serviceaccount:'+namespace+':'+identity] if as_identity else []
        result=subprocess.run(command+impersonation+['create','--dry-run=server','-f','-'],input=json.dumps(document),text=True,capture_output=True)
        assert result.returncode != 0 and expected in result.stderr, result.stderr
    try:
        apply({'apiVersion':'v1','kind':'ServiceAccount','metadata':{'name':identity,'namespace':namespace},'automountServiceAccountToken':False})
        apply({'apiVersion':'rbac.authorization.k8s.io/v1','kind':'Role','metadata':{'name':identity,'namespace':namespace},'rules':[{'apiGroups':[''],'resources':['pods','serviceaccounts'],'verbs':['create','update','patch']},{'apiGroups':['networking.k8s.io'],'resources':['networkpolicies'],'verbs':['create','update','patch']}]})
        apply({'apiVersion':'rbac.authorization.k8s.io/v1','kind':'RoleBinding','metadata':{'name':identity,'namespace':namespace},'subjects':[{'kind':'ServiceAccount','name':identity,'namespace':namespace}],'roleRef':{'apiGroup':'rbac.authorization.k8s.io','kind':'Role','name':identity}})
        pod={'apiVersion':'v1','kind':'Pod','metadata':{'name':'admission-negative','namespace':namespace},'spec':{'containers':[{'name':'test','image':'busybox:1.37.0','securityContext':{'allowPrivilegeEscalation':False,'capabilities':{'drop':['ALL']},'runAsNonRoot':True,'runAsUser':1000,'seccompProfile':{'type':'RuntimeDefault'}}}]}}
        reject(pod,'Applications cannot create or modify workloads',as_identity=True)
        reject({'apiVersion':'v1','kind':'ServiceAccount','metadata':{'name':'identity-negative','namespace':namespace}},'Applications cannot create or modify workloads',as_identity=True)
        reject({'apiVersion':'networking.k8s.io/v1','kind':'NetworkPolicy','metadata':{'name':'policy-negative','namespace':namespace},'spec':{'podSelector':{},'policyTypes':['Egress'],'egress':[{}]}},'Security configuration is owned',as_identity=True)
        pod['metadata']['annotations']={'ambient.istio.io/redirection':'disabled'}
        reject(pod,'Application pods cannot disable ambient capture')
        del pod['metadata']['annotations']
        pod['spec']['containers'][0]['securityContext']['privileged']=True
        reject(pod,'violates PodSecurity')
        reject({'apiVersion':'v1','kind':'Namespace','metadata':{'name':'homelab-admission-negative'}},'Application namespaces require restricted Pod Security and ambient enrollment')
        print('Admission denied workload/identity self-grants, policy self-grants, ambient opt-out, privileged containers, and insecure namespace creation')
    finally:
        for document in reversed(created):
            subprocess.run(command+['delete','--ignore-not-found','-f','-'],input=json.dumps(document),text=True,check=True,capture_output=True)


if __name__=='__main__':main()
