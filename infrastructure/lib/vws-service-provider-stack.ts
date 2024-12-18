import {Construct} from 'constructs';
import {SqsEventSource} from "aws-cdk-lib/aws-lambda-event-sources";
import {Queue} from "aws-cdk-lib/aws-sqs";
import {Vpc} from "aws-cdk-lib/aws-ec2";
import {Architecture, Code, Function, Runtime} from "aws-cdk-lib/aws-lambda";
import {AccountPrincipal, ManagedPolicy, PolicyStatement, Role, ServicePrincipal} from "aws-cdk-lib/aws-iam";
import {createVpcName, getAccountNameById, getVwsServiceConsumerRole, getVwsServiceQueueArn} from "../utils/helper";
import {Duration, SecretValue, Stack, StackProps} from "aws-cdk-lib";
import {Secret} from "aws-cdk-lib/aws-secretsmanager";
import {Bucket} from "aws-cdk-lib/aws-s3";
import {OutgoingProxy, OutgoingProxyCredentials} from "@vw-sre/vws-cdk";

const AtlasBasicResources: string[] = [
  "Cluster",
  "Project",
  "DatabaseUser",
  "ProjectIpAccessList",
];

export class VwsServiceProviderStack extends Stack {
  constructor(scope: Construct, id: string, props: StackProps) {
    super(scope, id, props);

    const resourceBucket = Bucket.fromBucketName(this, 'ResourceBucket', `mongodbatlas-cfn-resources-${getAccountNameById(this.account)}`);

    const vpc = Vpc.fromLookup(this, 'Vpc', {
      vpcName: createVpcName(this.account),
    });

    const proxy = new OutgoingProxy(this, 'OutgoingProxy', {
      allowedSuffixes: ['admin.vwapps.cloud'],
      allowedPorts: [443],
    })
    const proxyCredentials = new OutgoingProxyCredentials(this, 'OutgoingProxyCredentials', {
      instance: proxy,
      principals: [new AccountPrincipal(this.account)],
    })

    // Bootstrap MongoAtlas (as awscdk-resources-mongodbatlas.MongoAtlasBootstrap is not available)
    // Create a dummy Secret for MongoDB Atlas profile (https://github.com/mongodb/mongodbatlas-cloudformation-resources/tree/master?tab=readme-ov-file#2-configure-your-profile)
    const profileName = 'default'
    new Secret(this, 'MongoDBAtlasBootstrapSecret', {
      description: 'MongoDB Atlas profile secret',
      secretName: `cfn/atlas/profile/${profileName}`,
      secretObjectValue: {
        PublicKey: SecretValue.unsafePlainText('YourPublicKey'),
        PrivateKey: SecretValue.unsafePlainText('YourPrivateKey'),
      }
    })

    const vwsServiceQueue = Queue.fromQueueArn(this, 'MongoDBAtlasServiceProviderNotificationQueue', getVwsServiceQueueArn(this.account));

    const lambdaExecutionRole = new Role(this, 'MongoDBAtlasResourceHandlerRole', {
      roleName: 'MongoDBAtlasResourceHandlerRole',
      assumedBy: new ServicePrincipal('lambda.amazonaws.com'),
      managedPolicies: [
        ManagedPolicy.fromAwsManagedPolicyName('AWSCloudFormationFullAccess'),
        ManagedPolicy.fromAwsManagedPolicyName('CloudWatchLogsFullAccess'),
        ManagedPolicy.fromAwsManagedPolicyName('service-role/AWSLambdaVPCAccessExecutionRole'),
      ],
    });
    lambdaExecutionRole.addToPolicy(new PolicyStatement({
      actions: ['s3:*'],
      resources: [resourceBucket.arnForObjects('*')],
    }));
    lambdaExecutionRole.addToPolicy(new PolicyStatement({
      actions: ['kms:*'],
      resources: ['*'],
    }))

    const serviceConsumerRole = getVwsServiceConsumerRole(this.account);
    const serviceProxyUrl = `http://{{resolve:secretsmanager:${proxyCredentials.secret.secretArn}:SecretString:username:AWSCURRENT:}}:{{resolve:secretsmanager:${proxyCredentials.secret.secretArn}:SecretString:password:AWSCURRENT:}}@${proxy.dnsName}:8080`

    new Function(this, 'MongoDBAtlasResourceHandler', {
      vpc,
      role: lambdaExecutionRole,
      functionName: 'mongodb-atlas-resource-provider',
      description: 'Register all configured MongoDB Atlas CFN resources',
      runtime: Runtime.PROVIDED_AL2023,
      architecture: Architecture.ARM_64,
      code: Code.fromDockerBuild('./lib/vws-service-provider-handler'),
      handler: 'bootstrap',
      events: [
        new SqsEventSource(vwsServiceQueue),
      ],
      environment: {
        TYPES_TO_ACTIVATE: AtlasBasicResources.join(','),
        EXECUTION_ROLE_ARN_TEMPLATE: `arn:aws:iam::{ACCOUNT_ID}:role/vws/initializer/${serviceConsumerRole}`,
        SERVICES_PROXY: serviceProxyUrl,
        BUCKET_NAME: resourceBucket.bucketName,
      },
      timeout: Duration.seconds(90),
    });
  }
}
