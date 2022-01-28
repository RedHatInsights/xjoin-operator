"""create cats table

Revision ID: 6eb61f9873a9
Revises: 
Create Date: 2022-01-24 16:14:00.159545

"""
from alembic import op
import sqlalchemy as sa


# revision identifiers, used by Alembic.
from sqlalchemy.dialects import postgresql

revision = '6eb61f9873a9'
down_revision = None
branch_labels = None
depends_on = None


def upgrade():
    op.create_table(
        'cats',
        sa.Column('id', postgresql.UUID(), primary_key=True),
        sa.Column('name', sa.String(50), nullable=False),
    )
    op.execute('ALTER TABLE "cats" REPLICA IDENTITY FULL')


def downgrade():
    op.drop_table('cats')