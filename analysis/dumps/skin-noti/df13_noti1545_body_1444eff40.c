void __fastcall sub_1444EFF40(__int64 a1, unsigned int a2)
{
  __int64 **v4; // rsi
  __int64 *v5; // rbx
  int v6; // ebx
  int v7; // r8d
  __int64 *v8; // rax
  __int64 *v9; // rcx
  int v10; // r14d
  __int64 v11; // rcx
  __int64 v12; // rcx
  __int64 v13; // rax
  unsigned int *v14; // rax
  int v15; // ebx
  __int64 *v16; // rdi
  __int64 *v17; // rax
  __int64 *v18; // rcx
  __int64 v19; // rdx
  __int64 v20; // r8
  __int64 *v21; // rcx
  __int64 *v22; // rax
  __int128 v23; // [rsp+28h] [rbp-58h]
  __int64 v24; // [rsp+38h] [rbp-48h]
  __int128 v25; // [rsp+40h] [rbp-40h] BYREF
  __int64 v26; // [rsp+50h] [rbp-30h]
  int v27; // [rsp+60h] [rbp-20h] BYREF
  __int128 v28; // [rsp+64h] [rbp-1Ch]
  __int64 v29; // [rsp+74h] [rbp-Ch]
  unsigned __int16 v30; // [rsp+C8h] [rbp+48h] BYREF
  unsigned __int16 v31; // [rsp+D0h] [rbp+50h] BYREF
  __int64 v32; // [rsp+D8h] [rbp+58h] BYREF

  if ( a2 <= 9 )
  {
    v4 = (__int64 **)(16LL * (int)a2 + a1 + 136);
    v5 = *v4;
    sub_1401DBB80(v4, v4, (*v4)[1]);
    v5[1] = (__int64)v5;
    *v5 = (__int64)v5;
    v5[2] = (__int64)v5;
    v4[1] = 0;
    v30 = 0;
    v31 = 0;
    v32 = 0;
    sub_146EA1920(&v30);
    v6 = 0;
    if ( v30 )
    {
      do
      {
        if ( a2 == 4 )
        {
          sub_146EA0BA0(&v32);
          v7 = 0;
          HIDWORD(v32) = 0;
        }
        else
        {
          sub_146EA0BE0(&v32, 8);
          v7 = HIDWORD(v32);
        }
        if ( (_DWORD)v32 )
        {
          LODWORD(v24) = 0;
          BYTE4(v24) = 0;
          *(_QWORD *)&v23 = v32 | 0xFFFFFFFF00000000uLL;
          DWORD2(v23) = v7;
          BYTE12(v23) = 1;
          v8 = (__int64 *)(*v4)[1];
          v9 = *v4;
          while ( !*((_BYTE *)v8 + 25) )
          {
            if ( *((_DWORD *)v8 + 7) >= (int)v32 )
            {
              v9 = v8;
              v8 = (__int64 *)*v8;
            }
            else
            {
              v8 = (__int64 *)v8[2];
            }
          }
          if ( *((_BYTE *)v9 + 25) || (int)v32 < *((_DWORD *)v9 + 7) )
            v9 = *v4;
          if ( v9 == *v4 )
          {
            v27 = v32;
            v28 = v23;
            v29 = v24;
            sub_1444E8030(v4, &v25, &v27);
          }
          else
          {
            *((_DWORD *)v9 + 10) = v7;
            *((_BYTE *)v9 + 44) = 1;
          }
        }
        ++v6;
      }
      while ( v6 < v30 );
    }
    sub_146EA1920(&v31);
    v10 = 0;
    if ( v31 )
    {
      do
      {
        sub_146EA0BE0(&v32, 8);
        if ( (_DWORD)v32 )
        {
          *(_QWORD *)((char *)&v25 + 4) = 0xFFFFFFFFLL;
          BYTE12(v25) = 0;
          if ( sub_145EFAFB0(v11) )
          {
            v13 = sub_145EFAFB0(v12);
            v14 = (unsigned int *)(*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v13 + 7656LL))(v13);
            v15 = sub_1473A1120(a1 + 1328, *v14, (unsigned int)v32);
            LODWORD(v32) = v15;
          }
          else
          {
            v15 = v32;
          }
          LODWORD(v25) = v15;
          LODWORD(v26) = HIDWORD(v32);
          BYTE4(v26) = 1;
          v16 = *v4;
          v17 = (__int64 *)(*v4)[1];
          v18 = *v4;
          while ( !*((_BYTE *)v17 + 25) )
          {
            if ( *((_DWORD *)v17 + 7) >= v15 )
            {
              v18 = v17;
              v17 = (__int64 *)*v17;
            }
            else
            {
              v17 = (__int64 *)v17[2];
            }
          }
          if ( *((_BYTE *)v18 + 25) || v15 < *((_DWORD *)v18 + 7) || v18 == v16 )
          {
            v20 = sub_146E8BA20(56);
            *(_DWORD *)(v20 + 28) = v15;
            *(_OWORD *)(v20 + 32) = v25;
            *(_QWORD *)(v20 + 48) = v26;
            *(_QWORD *)v20 = v16;
            *(_QWORD *)(v20 + 8) = v16;
            *(_QWORD *)(v20 + 16) = v16;
            *(_WORD *)(v20 + 24) = 0;
            v21 = *v4;
            v22 = (__int64 *)(*v4)[1];
            *(_QWORD *)&v25 = v22;
            DWORD2(v25) = 0;
            if ( !*((_BYTE *)v22 + 25) )
            {
              v19 = *(unsigned int *)(v20 + 28);
              do
              {
                *(_QWORD *)&v25 = v22;
                if ( *((_DWORD *)v22 + 7) >= (int)v19 )
                {
                  DWORD2(v25) = 1;
                  v21 = v22;
                  v22 = (__int64 *)*v22;
                }
                else
                {
                  DWORD2(v25) = 0;
                  v22 = (__int64 *)v22[2];
                }
              }
              while ( !*((_BYTE *)v22 + 25) );
            }
            if ( *((_BYTE *)v21 + 25) || *(_DWORD *)(v20 + 28) < *((_DWORD *)v21 + 7) )
            {
              if ( v4[1] == (__int64 *)0x492492492492492LL )
                sub_14014F360(v21, v19);
              sub_14014F0E0(v4, &v25, v20);
            }
            else
            {
              sub_146E9F3A0(v20, 56);
            }
          }
          else
          {
            *((_DWORD *)v18 + 12) = HIDWORD(v32);
            *((_BYTE *)v18 + 52) = 1;
          }
          if ( a2 == 5 )
            sub_146A19E60((unsigned int)v32, 1);
        }
        ++v10;
      }
      while ( v10 < v31 );
    }
  }
}
