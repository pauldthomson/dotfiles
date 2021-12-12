kube_debug ()
{
    IMAGE=${IMAGE:-"519920317464.dkr.ecr.ap-southeast-2.amazonaws.com/corectl:v1.0.0-alpha.134"};
    SA="${SA:-default}";
    NAME="$(whoami)-debug";
    kubectl run ${NAME} -it -n payments --command=true --rm=true --grace-period 0 --image ${IMAGE} --labels "app=${NAME},tyro-team=architecture" /bin/sh
}
