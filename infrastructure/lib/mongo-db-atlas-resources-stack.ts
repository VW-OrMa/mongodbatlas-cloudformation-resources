import {Construct} from 'constructs';
import {Vpc} from "aws-cdk-lib/aws-ec2";
import {createVpcName, getAccountNameById} from "../utils/helper";
import {DockerImage, RemovalPolicy, Size, Stack, StackProps} from "aws-cdk-lib";
import {Bucket, IBucket} from "aws-cdk-lib/aws-s3";
import {BucketDeployment, Source} from "aws-cdk-lib/aws-s3-deployment";
import {PolicyStatement, ServicePrincipal} from "aws-cdk-lib/aws-iam";
import {LogGroup, RetentionDays} from "aws-cdk-lib/aws-logs";
import * as path from "node:path";

export class MongoDbAtlasResourcesStack extends Stack {

  readonly customServiceProviderBucket: IBucket;

  constructor(scope: Construct, id: string, props?: StackProps) {
    super(scope, id, props);

    const vpc = Vpc.fromLookup(this, 'Vpc', {
      vpcName: createVpcName(this.account),
    });

    // Upload all mongodb cfn resources to S3
    const customServiceProviderBucket = new Bucket(this, 'CustomServiceProviderBucket', {
      bucketName: `mongodbatlas-cfn-resources-${getAccountNameById(this.account)}`,
      removalPolicy: RemovalPolicy.DESTROY
    });
    customServiceProviderBucket.addToResourcePolicy(new PolicyStatement({
      actions: ['s3:*'],
      resources: [customServiceProviderBucket.arnForObjects('*')],
      principals: [
        new ServicePrincipal('lambda.amazonaws.com'),
        new ServicePrincipal('cloudformation.amazonaws.com'),
      ],
    }));
    this.customServiceProviderBucket = customServiceProviderBucket

    const bucketDeployment = new BucketDeployment(this, 'MongoDBAtlasResourcesDeployment', {
      vpc,
      sources: [Source.asset(path.join(__dirname, '../../cfn-resources'), {
        bundling: {
          image: DockerImage.fromBuild(path.join(__dirname, '../../')),
        },
      })],
      ephemeralStorageSize: Size.gibibytes(2), // Assets have about 380 MB at the moment - unpacking and caching will take space
      memoryLimit: 1024, // Assets have about 380 MB at the moment - unpacking and caching will take space
      destinationBucket: customServiceProviderBucket,
      retainOnDelete: false,
      logGroup: new LogGroup(this, 'MongoDBAtlasResourcesDeploymentLogGroup', {
        logGroupName: '/aws/lambda/mongodb-cfn-resources-bucket-deployment',
        removalPolicy: RemovalPolicy.DESTROY,
        retention: RetentionDays.ONE_MONTH
      })
    });

    // the cdk asset bucket is encrypted
    bucketDeployment.handlerRole.addToPrincipalPolicy(new PolicyStatement({
      resources: ['*'],
      actions: ['kms:Decrypt'],
    }));
  }
}
