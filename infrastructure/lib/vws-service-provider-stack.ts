import {Construct} from 'constructs';
import {SqsEventSource} from "aws-cdk-lib/aws-lambda-event-sources";
import {Queue} from "aws-cdk-lib/aws-sqs";
import {Vpc} from "aws-cdk-lib/aws-ec2";
import {Architecture, Code, Function, Runtime} from "aws-cdk-lib/aws-lambda";
import {ManagedPolicy, PolicyDocument, PolicyStatement, Role, ServicePrincipal} from "aws-cdk-lib/aws-iam";
import {createVpcName, getVwsServiceQueueArn} from "../utils/helper";
import {Duration, SecretValue, Stack, StackProps} from "aws-cdk-lib";
import {Secret} from "aws-cdk-lib/aws-secretsmanager";
import {IBucket} from "aws-cdk-lib/aws-s3";

const AtlasBasicResources: string[] = [
  "Cluster",
  "Project",
  "DatabaseUser",
  "ProjectIpAccessList",
];

interface VwsServiceProviderStackProps extends StackProps {
  resourceBucket: IBucket
}

export class VwsServiceProviderStack extends Stack {
  constructor(scope: Construct, id: string, props: VwsServiceProviderStackProps) {
    super(scope, id, props);

    const vpc = Vpc.fromLookup(this, 'Vpc', {
      vpcName: createVpcName(this.account),
    });

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

    // Create mongo resources execution role
    const mongoResourcesExecutionRole = new Role(this, 'MongoDBAtlasResourcesExecutionRole', {
      roleName: 'MongoDBAtlasResourcesExecutionRole',
      assumedBy: new ServicePrincipal('resources.cloudformation.amazonaws.com'),
      inlinePolicies: {
        ResourceTypePolicy: new PolicyDocument({
          statements: [
            new PolicyStatement({
              resources: ['*'],
              actions: [
                // "ec2:DeleteSecurityGroup",
                // "ec2:DescribeAccountAttributes",
                // "ec2:DescribeImages",
                // "ec2:DescribeInstances",
                // "ec2:DescribeInternetGateways",
                // "ec2:DescribeRouteTables",
                // "ec2:DescribeSecurityGroups",
                // "ec2:DescribeSubnets",
                // "ec2:DescribeTags",
                // "ec2:DescribeVolumes",
                // "ec2:DescribeVpcAttribute",
                // "ec2:DescribeVpcClassicLink",
                // "ec2:DescribeVpcClassicLinkDnsSupport",
                // "ec2:DescribeVpcEndpoints",
                // "ec2:DescribeVpcs",
                // "ec2:RevokeSecurityGroupIngress",
                // "ec2:TerminateInstances",
                // "elasticloadbalancing:*",
                "iam:GetRole",
                "iam:GetRolePolicy",
                "iam:GetUser",
                "iam:ListAccessKeys",
                "iam:PassRole",
                // "route53:ChangeResourceRecordSets",
                // "route53:GetChange",
                // "route53:GetHostedZone",
                // "route53:ListHostedZones",
                // "route53:ListHostedZonesByName",
                // "route53:ListQueryLoggingConfigs",
                // "route53:ListResourceRecordSets",
                "s3:*",
                // "secretsmanager:CreateSecret",
                // "secretsmanager:DeleteSecret",
                // "secretsmanager:DescribeSecret",
                // "secretsmanager:GetSecretValue",
                // "secretsmanager:ListSecrets",
                // "secretsmanager:PutSecretValue",
                // "secretsmanager:TagResource",
                "ssm:*",
                "tag:GetResources"
              ]
            })
          ]
        })
      },
      description: 'Role to execute MongoDB Atlas resources',
    });
    mongoResourcesExecutionRole.assumeRolePolicy?.addStatements(new PolicyStatement({
      actions: ['sts:AssumeRole'],
      principals: [new ServicePrincipal('resources.cloudformation.amazonaws.com')],
    }));

    // provide service role and notification handler queue of the custom vws service provider
    const vwsServiceRole = Role.fromRoleArn(this, 'MongoDBAtlasServiceProviderRole', `arn:aws:iam::${this.account}:role/vws/initializer/vws-init-1d0a77-CloudFormationRegistration`)
    const vwsServiceQueue = Queue.fromQueueArn(this, 'MongoDBAtlasServiceProviderNotificationQueue', getVwsServiceQueueArn(this.account));

    const lambdaExecutionRole = new Role(this, 'MongoDBAtlasResourceHandlerRole', {
      roleName: 'MongoDBAtlasResourceHandlerRole',
      assumedBy: new ServicePrincipal('lambda.amazonaws.com'),
      managedPolicies: [
        ManagedPolicy.fromAwsManagedPolicyName('AWSCloudFormationFullAccess'),
        ManagedPolicy.fromAwsManagedPolicyName('CloudWatchLogsFullAccess'),
      ],
    });
    lambdaExecutionRole.addToPolicy(new PolicyStatement({
      actions: ['sts:AssumeRole'],
      resources: [vwsServiceRole.roleArn]
    }));
    lambdaExecutionRole.addToPolicy(new PolicyStatement({
      actions: ['iam:PassRole'],
      resources: [mongoResourcesExecutionRole.roleArn],
    }));
    lambdaExecutionRole.addToPolicy(new PolicyStatement({
      actions: ['s3:*'],
      resources: [props.resourceBucket.arnForObjects('*')],
    }));
    lambdaExecutionRole.addToPolicy(new PolicyStatement({
      actions: ['kms:*'],
      resources: ['*'],
    }))

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
        EXECUTION_ROLE_ARN: mongoResourcesExecutionRole.roleArn,
        BUCKET_NAME: props.resourceBucket.bucketName,
      },
      timeout: Duration.minutes(5),
    });
  }
}
